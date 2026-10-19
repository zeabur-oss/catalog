package recs

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	smokeNamespace = "recs-smoke"
	settle         = time.Minute // after install, before measuring
)

// Tester installs components on the current kubectl context one at a time.
type Tester struct {
	Catalog   *Catalog
	Profile   string
	Overrides map[string]map[string]string // test-only values per component and release
	Unready   map[string]bool              // components that cannot become ready without credentials
	// Test-only text replacements in manifests, per component and release.
	Replacements map[string]map[string]map[string]string
	Previous     map[string]*Component // last release's components, for upgrade tests
}

// Result is the outcome for one component.
type Result struct {
	ID            string
	IdleMemoryMiB float64
	IdleCPUm      float64
	UpgradedFrom  string
	Unmeasured    bool // installed without waiting; nothing ran long enough to measure
	Err           error
}

// run tracks one test so it can be undone in reverse order.
type run struct {
	undo          []func() error
	installed     map[string]bool     // component IDs
	releases      map[string]bool     // component/release with an undo step
	contributions map[string][]string // component/release -> extra values
}

func kubectl(args ...string) (string, error) { return Run(nil, "kubectl", args...) }

// Test installs id with its dependencies, measures and smoke tests it,
// uninstalls everything and fails on anything left behind.
func (t *Tester) Test(id string) (res Result) {
	res.ID = id
	order, err := t.Catalog.InstallOrder([]string{id}, t.Profile)
	if err != nil {
		res.Err = err
		return
	}
	before, err := inventory()
	if err != nil {
		res.Err = err
		return
	}
	r := &run{installed: map[string]bool{}, releases: map[string]bool{}, contributions: map[string][]string{}}
	defer func() {
		var errs []error
		for _, undo := range slices.Backward(r.undo) {
			errs = append(errs, undo())
		}
		after, err := inventory()
		errs = append(errs, err)
		added := addedSince(before, after)
		if left := residue(order, added); len(left) > 0 {
			errs = append(errs, fmt.Errorf("left behind: %s", strings.Join(left, ", ")))
		}
		for _, e := range added { // clean up accepted leftovers for the next test
			kind, name, _ := strings.Cut(e, " ")
			ns, n, _ := strings.Cut(name, "/")
			kubectl("delete", kind, n, "--ignore-not-found", "--wait=false", "-n", cmp.Or(ns, "default"))
		}
		res.Err = errors.Join(res.Err, errors.Join(errs...))
	}()

	for _, c := range order {
		target := c.ID == id
		var existing map[string]bool
		if target {
			if existing, err = podNames(); err != nil {
				res.Err = err
				return
			}
			if prev, ok := t.Previous[id]; ok && versions(prev) != versions(c) {
				slog.Info("installing previous release for the upgrade test", "component", id, "versions", versions(prev))
				if err := t.install(r, prev); err != nil {
					res.Err = fmt.Errorf("install previous: %w", err)
					return
				}
				res.UpgradedFrom = versions(prev)
			}
		}
		if err := t.install(r, c); err != nil {
			res.Err = fmt.Errorf("install %s: %w", c.ID, err)
			return
		}
		if !target {
			continue
		}
		if t.Unready[id] {
			slog.Info("installed without credentials; not measured", "component", id)
			res.Unmeasured = true
			if err := smoke(c); err != nil {
				res.Err = fmt.Errorf("smoke: %w", err)
			}
			return res
		}
		slog.Info("installed; settling", "component", id, "for", settle)
		time.Sleep(settle)
		// Only new pods in the component's own namespaces are its cost.
		namespaces := map[string]bool{}
		for _, rel := range c.Releases {
			namespaces[rel.Namespace] = true
		}
		res.IdleMemoryMiB, res.IdleCPUm, err = sampleUsage(func(ns, name string) bool {
			return namespaces[ns] && !existing[ns+"/"+name]
		})
		if err != nil {
			res.Err = err
			return
		}
		slog.Info("measured", "component", id, "memoryMiB", int(res.IdleMemoryMiB), "cpuM", int(res.IdleCPUm))
		if err := smoke(c); err != nil {
			res.Err = fmt.Errorf("smoke: %w", err)
			return
		}
	}
	return res
}

func versions(c *Component) string {
	var v []string
	for _, r := range c.Releases {
		if r.Version != "" {
			v = append(v, r.Name+"@"+r.Version)
		}
	}
	return strings.Join(v, ",")
}

// install installs a component's secrets and releases, then re-applies the
// releases it contributes values to.
func (t *Tester) install(r *run, c *Component) error {
	if err := createSecrets(r, c); err != nil {
		return err
	}
	for _, rel := range c.Releases {
		if len(rel.SkipIfPresent) > 0 && !r.releases[c.ID+"/"+rel.Name] && crdsExist(rel.SkipIfPresent) {
			slog.Info("already present, skipped", "component", c.ID, "release", rel.Name)
			continue
		}
		if err := t.installRelease(r, c, rel); err != nil {
			return err
		}
	}
	r.installed[c.ID] = true
	for _, ct := range c.Contributes {
		key := ct.Target + "/" + ct.Release
		r.contributions[key] = append(r.contributions[key], ct.Values)
		if !r.installed[ct.Target] {
			continue
		}
		target, _ := t.Catalog.Get(ct.Target)
		rel, _ := target.Release(ct.Release)
		slog.Info("adding contributed values", "component", c.ID, "target", key)
		if err := t.installRelease(r, target, rel); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tester) installRelease(r *run, c *Component, rel Release) error {
	if err := ensureNamespace(r, rel.Namespace); err != nil {
		return err
	}
	key := c.ID + "/" + rel.Name
	if rel.Kind() != "helm" {
		manifest, err := releaseManifest(rel)
		if err != nil {
			return err
		}
		if manifest, err = replace(manifest, t.Replacements[c.ID][rel.Name]); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		slog.Info("applying", "component", c.ID, "release", rel.Name)
		if err := retry(3*time.Minute, func() error {
			_, err := Run(manifest, "kubectl", "apply", "--server-side", "--force-conflicts", "-f", "-")
			return err
		}); err != nil {
			return err
		}
		if !r.releases[key] {
			r.releases[key] = true
			r.undo = append(r.undo, func() error {
				_, err := Run(manifest, "kubectl", "delete", "-f", "-", "--ignore-not-found", "--wait", "--timeout=5m")
				return err
			})
		}
		return waitReady(manifest, rel.Namespace)
	}

	chart, err := pull(rel)
	if err != nil {
		return err
	}
	vals, err := c.values(rel, t.Profile, c.testInputs())
	if err != nil {
		return err
	}
	docs := []map[string]any{vals}
	for _, extra := range append(r.contributions[key], t.Overrides[c.ID][rel.Name]) {
		m, err := ParseValues(extra)
		if err != nil {
			return err
		}
		docs = append(docs, m)
	}
	files, cleanup, err := valuesFiles(docs...)
	if err != nil {
		return err
	}
	defer cleanup()
	slog.Info("helm upgrade --install", "component", c.ID, "release", rel.Name, "chart", rel.Chart, "version", rel.Version)
	args := []string{"upgrade", "--install", rel.HelmName(), chart, "--namespace", rel.Namespace}
	if !t.Unready[c.ID] { // its workloads crash without real credentials
		args = append(args, "--wait", "--timeout", "10m")
	}
	args = append(args, files...)
	if _, err := Run(nil, "helm", args...); err != nil {
		return err
	}
	if !r.releases[key] {
		r.releases[key] = true
		r.undo = append(r.undo, func() error {
			_, err := Run(nil, "helm", "uninstall", rel.HelmName(), "--namespace", rel.Namespace, "--wait", "--timeout", "10m")
			return err
		})
	}
	return nil
}

// replace applies test-only replacements to a manifest, in key order. A
// replacement that matches nothing fails, so it cannot go stale unnoticed.
func replace(manifest []byte, replacements map[string]string) ([]byte, error) {
	out := string(manifest)
	for _, old := range slices.Sorted(maps.Keys(replacements)) {
		if !strings.Contains(out, old) {
			return nil, fmt.Errorf("replacement %q matches nothing", old)
		}
		out = strings.ReplaceAll(out, old, replacements[old])
	}
	return []byte(out), nil
}

// createSecrets creates the Secrets that inputs live in, one per name, with
// every key. Existing Secrets are kept, so an upgrade test reuses them.
func createSecrets(r *run, c *Component) error {
	inputs := c.testInputs()
	keys := map[SecretRef][]string{} // Key unset: one entry per Secret
	for _, in := range c.Inputs {
		if in.Secret == nil {
			continue
		}
		value := fmt.Sprint(inputs[in.Name])
		if in.Generate == "random" {
			b := make([]byte, 64)
			rand.Read(b)
			value = hex.EncodeToString(b)
		} else if inputs[in.Name] == nil {
			continue
		}
		ref := SecretRef{Name: in.Secret.Name, Namespace: in.Secret.Namespace}
		keys[ref] = append(keys[ref], "--from-literal="+in.Secret.Key+"="+value)
	}
	for ref, literals := range keys {
		if _, err := kubectl("-n", ref.Namespace, "get", "secret", ref.Name); err == nil {
			continue
		}
		if err := ensureNamespace(r, ref.Namespace); err != nil {
			return err
		}
		if _, err := kubectl(append([]string{"-n", ref.Namespace, "create", "secret", "generic", ref.Name}, literals...)...); err != nil {
			return err
		}
		r.undo = append(r.undo, func() error {
			_, err := kubectl("-n", ref.Namespace, "delete", "secret", ref.Name, "--ignore-not-found")
			return err
		})
	}
	return nil
}

func ensureNamespace(r *run, ns string) error {
	if _, err := kubectl("get", "namespace", ns); err == nil {
		return nil
	}
	if _, err := kubectl("create", "namespace", ns); err != nil {
		return err
	}
	r.undo = append(r.undo, func() error {
		_, err := kubectl("delete", "namespace", ns, "--ignore-not-found", "--wait", "--timeout=10m")
		return err
	})
	return nil
}

func crdsExist(crds []string) bool {
	_, err := kubectl(append([]string{"get", "crd"}, crds...)...)
	return err == nil
}

// waitReady waits for a manifest's CRDs and workloads.
func waitReady(manifest []byte, defaultNS string) error {
	for _, doc := range strings.Split(string(manifest), "\n---") {
		var o struct {
			Kind     string
			Metadata struct{ Name, Namespace string }
		}
		if yaml.Unmarshal([]byte(doc), &o) != nil {
			continue
		}
		var err error
		switch o.Kind {
		case "CustomResourceDefinition":
			_, err = kubectl("wait", "--for=condition=Established", "crd/"+o.Metadata.Name, "--timeout=2m")
		case "Deployment", "StatefulSet", "DaemonSet":
			_, err = kubectl("-n", cmp.Or(o.Metadata.Namespace, defaultNS), "rollout", "status",
				strings.ToLower(o.Kind)+"/"+o.Metadata.Name, "--timeout=10m")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func smoke(c *Component) error {
	if len(c.Smoke) == 0 {
		return nil
	}
	if _, err := kubectl("create", "namespace", smokeNamespace); err != nil {
		return err
	}
	defer kubectl("delete", "namespace", smokeNamespace, "--wait", "--timeout=5m")
	for _, s := range c.Smoke {
		slog.Info("smoke", "component", c.ID, "check", s.Name)
		if s.Manifest != "" {
			if err := retry(2*time.Minute, func() error {
				_, err := Run([]byte(s.Manifest), "kubectl", "apply", "-f", "-")
				return err
			}); err != nil {
				return fmt.Errorf("%s: %w", s.Name, err)
			}
		}
		for _, w := range s.Wait {
			args := append(append([]string{"wait"}, strings.Fields(w)...), "--timeout="+cmp.Or(s.Timeout, "180s"))
			if _, err := kubectl(args...); err != nil {
				return fmt.Errorf("%s: %w", s.Name, err)
			}
		}
	}
	return nil
}

// retry runs f every 5s until it succeeds or timeout passes; webhooks often
// need a moment after their Deployment is ready.
func retry(timeout time.Duration, f func() error) error {
	deadline := time.Now().Add(timeout)
	for {
		err := f()
		if err == nil || time.Now().After(deadline) {
			return err
		}
		time.Sleep(5 * time.Second)
	}
}

// sampleUsage returns the median working set (MiB) and CPU (millicores) of
// matching pods over five samples, from the kubelet summary API.
func sampleUsage(match func(ns, name string) bool) (mem, cpu float64, err error) {
	var mems, cpus []float64
	for i := range 5 {
		if i > 0 {
			time.Sleep(10 * time.Second)
		}
		m, c, err := podUsage(match)
		if err != nil {
			return 0, 0, err
		}
		mems, cpus = append(mems, m), append(cpus, c)
	}
	slices.Sort(mems)
	slices.Sort(cpus)
	return mems[2], cpus[2], nil
}

func podUsage(match func(ns, name string) bool) (mem, cpu float64, err error) {
	nodes, err := kubectl("get", "nodes", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return 0, 0, err
	}
	for _, node := range strings.Fields(nodes) {
		raw, err := kubectl("get", "--raw", "/api/v1/nodes/"+node+"/proxy/stats/summary")
		if err != nil {
			return 0, 0, err
		}
		var s struct {
			Pods []struct {
				PodRef struct{ Namespace, Name string }
				Memory struct{ WorkingSetBytes float64 }
				CPU    struct{ UsageNanoCores float64 }
			}
		}
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return 0, 0, err
		}
		for _, p := range s.Pods {
			if match(p.PodRef.Namespace, p.PodRef.Name) {
				mem += p.Memory.WorkingSetBytes / (1 << 20)
				cpu += p.CPU.UsageNanoCores / 1e6
			}
		}
	}
	return mem, cpu, nil
}

// podNames returns "namespace/name" for every pod. Names, not UIDs: the
// kubelet reports static pods under a UID the API server never shows.
func podNames() (map[string]bool, error) {
	out, err := kubectl("get", "pods", "-A", "-o", `jsonpath={range .items[*]}{.metadata.namespace}/{.metadata.name}{" "}{end}`)
	names := map[string]bool{}
	for _, n := range strings.Fields(out) {
		names[n] = true
	}
	return names, err
}

// ignored resources churn on their own or belong to tracked objects.
var ignored = []string{
	"events", "events.events.k8s.io", "leases.coordination.k8s.io", "endpoints",
	"endpointslices.discovery.k8s.io", "pods", "replicasets.apps", "controllerrevisions.apps",
	"jobs.batch", "componentstatuses", "certificatesigningrequests.certificates.k8s.io",
	"nodes.metrics.k8s.io", "pods.metrics.k8s.io", "selfsubjectreviews.authentication.k8s.io",
}

// inventory lists every object in the cluster as "resource namespace/name".
func inventory() (map[string]bool, error) {
	out, err := kubectl("api-resources", "--verbs=list", "-o", "name")
	if out == "" { // a down aggregated API fails the command but the rest still prints
		return nil, err
	}
	inv := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, res := range strings.Fields(out) {
		if slices.Contains(ignored, res) {
			continue
		}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			list, err := kubectl("get", res, "-A", "--ignore-not-found",
				"-o", `jsonpath={range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}`)
			if err != nil {
				return // the API went away with its CRD
			}
			mu.Lock()
			defer mu.Unlock()
			for _, line := range strings.Fields(list) {
				inv[res+" "+line] = true
			}
		})
	}
	wg.Wait()
	return inv, nil
}

func addedSince(before, after map[string]bool) []string {
	var out []string
	for e := range after {
		if !before[e] {
			out = append(out, e)
		}
	}
	slices.Sort(out)
	return out
}

// residue drops declared leftovers from what an uninstall left behind. Kept
// CRDs also keep their objects and API registrations.
func residue(order []*Component, added []string) []string {
	allowed := map[string]bool{}
	for _, c := range order {
		for _, l := range c.Leftovers {
			allowed[l] = true
		}
	}
	var groups []string
	for _, e := range added {
		if crd, ok := strings.CutPrefix(e, "customresourcedefinitions.apiextensions.k8s.io /"); ok && allowed["customresourcedefinitions"] {
			_, group, _ := strings.Cut(crd, ".")
			groups = append(groups, "."+group)
		}
	}
	keptGroup := func(s string) bool {
		return slices.ContainsFunc(groups, func(g string) bool { return strings.HasSuffix(s, g) })
	}
	var out []string
	for _, e := range added {
		res, name, _ := strings.Cut(e, " ")
		short, _, _ := strings.Cut(res, ".")
		_, obj, _ := strings.Cut(name, "/")
		if allowed[short] || allowed[short+"/"+obj] || keptGroup(res) ||
			res == "apiservices.apiregistration.k8s.io" && keptGroup(obj) {
			continue
		}
		out = append(out, e)
	}
	return out
}
