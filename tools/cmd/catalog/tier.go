package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
	"go.yaml.in/yaml/v3"
)

// Tiers. L0 runs on every pull request: static checks, then a server-side dry
// run on a local API server with only the CRDs and no nodes. L1 runs on main
// only: it installs and smoke tests what changed, single-node amd64, in
// groups that share their dependencies. L2 runs nightly
// and before a release: every profile and architecture in a fresh cluster
// each, with idle usage, uninstall, residue and upgrade checks. With -cluster
// current, L1 and L2 use the current cluster instead of kind.
var tiers = []string{"l0", "l1", "l2"}

// row is one line of the tier report.
type row struct {
	subject string
	status  string
	timings catalog.Timings
	err     error
}

func report(title string, rows []row) error {
	fmt.Printf("\n%s:\n", title)
	var errs []error
	var total time.Duration
	for _, r := range rows {
		status := cmp.Or(r.status, "PASS")
		if r.err != nil {
			status = "FAIL " + r.err.Error()
			errs = append(errs, fmt.Errorf("%s: %w", r.subject, r.err))
		}
		var took time.Duration
		for _, p := range r.timings {
			took += p.Took
		}
		total += took
		fmt.Printf("  %-48s %s\n  %-48s %s (total %s)\n", r.subject, status, "", r.timings, took)
	}
	fmt.Printf("  total %s\n", total)
	return errors.Join(errs...)
}

// selector returns a function that reports whether a test runs: its subject
// or a component it installs changed since ref. An empty ref, or a change to
// a shared file, selects everything.
func selector(r *repo, since string) (func(id string, order []*catalog.Component) bool, error) {
	var want map[string]bool // nil selects everything
	if since != "" && strings.Trim(since, "0") != "" {
		out, err := catalog.Run(nil, "git", "diff", "--name-only", since, "HEAD")
		if err != nil {
			return nil, err
		}
		want = changed(r, strings.Fields(out))
	}
	return func(id string, order []*catalog.Component) bool {
		return want == nil || want[id] || slices.ContainsFunc(order, func(o *catalog.Component) bool { return want[o.ID] })
	}, nil
}

// group is what one L1 cluster tests, in order. Its members share dependencies.
type group struct {
	IDs  []string
	deps map[string]bool
	cost int
}

// groupCost caps the cost of one L1 group. A component costs 1, and a template
// 1 per service and per component it requires. On a 4-CPU VM, one unit took
// about 40 seconds.
const groupCost = 20

// l1Groups returns the single-node subjects that sel selects, in groups that
// share dependencies. The grouping is deterministic.
func l1Groups(r *repo, sel func(string, []*catalog.Component) bool) ([]*group, error) {
	items, err := subjects(r, templateProfile, sel)
	if err != nil {
		return nil, err
	}
	// Place the items with the most dependencies first.
	slices.SortStableFunc(items, func(a, b subject) int { return len(b.deps) - len(a.deps) })
	var groups []*group
	for _, it := range items {
		var best *group
		bestNew := len(it.deps) + 1
		for _, g := range groups {
			if g.cost+it.cost > groupCost {
				continue
			}
			n := 0
			for _, d := range it.deps {
				if !g.deps[d] {
					n++
				}
			}
			if n < bestNew {
				best, bestNew = g, n
			}
		}
		if best == nil {
			best = &group{deps: map[string]bool{}}
			groups = append(groups, best)
		}
		best.IDs = append(best.IDs, it.id)
		best.cost += it.cost
		for _, d := range it.deps {
			best.deps[d] = true
		}
	}
	return groups, nil
}

func ids(cs []*catalog.Component) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

// templateRefs returns what a template's e2e installs: its requires, and the
// SeaweedFS component when its test values use the S3 gateway.
func templateRefs(r *repo, t *templates.Template) ([]string, error) {
	values, err := e2eValues(r.templates, t)
	if err != nil {
		return nil, err
	}
	refs := slices.Clone(t.Requires)
	if slices.ContainsFunc(slices.Collect(maps.Values(values.Inputs)), func(v string) bool { return strings.Contains(v, ".S3") }) {
		refs = append(refs, templates.S3Component)
	}
	return refs, nil
}

// subject is one component or template test.
type subject struct {
	id    string
	order []*catalog.Component // what the test installs, in order
	deps  []string             // the IDs in order, without the component under test
	cost  int                  // see groupCost
}

// subjects returns every component that profile supports and, on
// single-node, every template, that sel selects.
func subjects(r *repo, profile string, sel func(string, []*catalog.Component) bool) ([]subject, error) {
	var out []subject
	for _, c := range r.cat.Components {
		if c.Defaults[profile] == "unsupported" {
			continue
		}
		order, err := r.cat.InstallOrder([]string{c.ID}, profile)
		if err != nil {
			return nil, err
		}
		if sel(c.ID, order) {
			out = append(out, subject{c.ID, order, ids(order[:len(order)-1]), 1})
		}
	}
	if profile != templateProfile {
		return out, nil
	}
	for _, t := range r.templates {
		order, err := installOrder(r, t.ID, profile)
		if err != nil {
			return nil, err
		}
		if sel(t.ID, order) {
			out = append(out, subject{t.ID, order, ids(order), len(t.Services) + len(t.Requires)})
		}
	}
	return out, nil
}

// kubesoloProfile is the host profile that installs KubeSolo.
const kubesoloProfile = "kubesolo"

// l2Profiles returns the catalog profiles, then the host profiles.
func l2Profiles() []string { return append(slices.Clone(catalog.Profiles), hostProfiles...) }

// profileSubjects is subjects for a catalog profile or a host profile. A host
// profile tests components only.
func profileSubjects(r *repo, profile string, sel func(string, []*catalog.Component) bool) ([]subject, error) {
	if !slices.Contains(hostProfiles, profile) {
		return subjects(r, profile, sel)
	}
	subs, err := subjects(r, templateProfile, sel)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(subs, func(s subject) bool {
		c, ok := r.cat.Get(s.id)
		return !ok || !hostRuns(profile, c)
	}), nil
}

// selection returns a function that reports whether a test runs: one that
// args names or, without args, one that changed since ref (see selector).
func selection(r *repo, since string, args []string) (func(string, []*catalog.Component) bool, error) {
	if len(args) == 0 {
		return selector(r, since)
	}
	for _, id := range args {
		if _, err := installOrder(r, id, templateProfile); err != nil {
			return nil, err
		}
	}
	return func(id string, _ []*catalog.Component) bool { return slices.Contains(args, id) }, nil
}

// installOrder returns what testing id installs. Component IDs contain a
// slash and template IDs do not.
func installOrder(r *repo, id, profile string) ([]*catalog.Component, error) {
	if strings.Contains(id, "/") {
		if _, ok := r.cat.Get(id); !ok {
			return nil, fmt.Errorf("unknown component %q", id)
		}
		return r.cat.InstallOrder([]string{id}, profile)
	}
	t, err := templates.Get(r.templates, id)
	if err != nil {
		return nil, err
	}
	refs, err := templateRefs(r, t)
	if err != nil {
		return nil, err
	}
	return r.cat.InstallOrder(refs, profile)
}

type tierOptions struct {
	profile, since, previous string
	write, keep              bool
	current                  bool // test on the cluster that KUBECONFIG names instead of kind
}

func runTier(r *repo, tier string, o tierOptions, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	switch tier {
	case "l0":
		return l0(r, cfg, o)
	case "l1":
		return l1(r, cfg, o, args)
	case "l2":
		return l2(r, cfg, o, args)
	}
	return fmt.Errorf("unknown -tier %q; use %s", tier, strings.Join(tiers, ", "))
}

// l0 validates, renders and lints everything, then sends every rendered
// component and template to a local API server with --dry-run=server.
func l0(r *repo, cfg config, o tierOptions) error {
	var ts catalog.Timings
	step := func(name string, f func() error) error {
		start := time.Now()
		err := f()
		ts.Since(name, start)
		if err != nil {
			report("l0", []row{{subject: "l0", timings: ts, err: fmt.Errorf("%s: %w", name, err)}})
		}
		return err
	}
	if err := step("validate", func() error {
		return errors.Join(catalog.Check(".", r.cat), templates.Check(".", r.cat, r.templates))
	}); err != nil {
		return err
	}
	out := "out/render"
	if err := step("render", func() error { return renderComponents(r.cat, r.cat.Components, out, cfg.Versions) }); err != nil {
		return err
	}
	if err := step("template lint", func() error { return lintTemplates(r, out+"-schemas", cfg.Versions) }); err != nil {
		return err
	}
	var api *apiServer
	if err := step("api server", func() (err error) {
		api, err = startAPIServer()
		return err
	}); err != nil {
		return err
	}
	if !o.keep {
		defer api.stop()
	}
	server, err := api.version()
	if err != nil {
		return err
	}
	minor := func(v string) string { return strings.Join(strings.SplitN(v, ".", 3)[:2], ".") }
	i := slices.IndexFunc(cfg.Versions, func(v string) bool { return minor(v) == minor(server) })
	if i < 0 {
		return fmt.Errorf("the API server is Kubernetes %s; config/kubernetes.yaml renders no version with the same minor", server)
	}
	kv := cfg.Versions[i]
	err = step("dry run", func() error { return dryRun(r, filepath.Join(out, kv)) })
	if err == nil {
		err = report("l0", []row{{subject: "l0", timings: ts}})
	}
	return err
}

// lintTemplates runs the safety rules and kubeconform on every template,
// rendered with its e2e values. schemas holds the CRD schemas that render wrote.
func lintTemplates(r *repo, schemas string, versions []string) error {
	errs := make([]error, len(r.templates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, t := range r.templates {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			v, err := e2eValues(r.templates, t)
			if err != nil {
				errs[i] = err
				return
			}
			manifest, err := templates.E2EManifest(t, v)
			if err == nil {
				err = templates.Lint(t, manifest, templates.Namespace(t), schemas, versions)
			}
			if err != nil {
				errs[i] = fmt.Errorf("templates/%s: %w", t.ID, err)
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// dryRun applies every CRD under dir, creates the namespaces, then applies
// every rendered component and template with --dry-run=server. The API
// server checks fields, defaults, CEL rules and Pod Security; nothing runs.
// A warning on a template fails it.
func dryRun(r *repo, dir string) error {
	type file struct {
		subject, namespace string
		data               []byte
		strict             bool
	}
	var files []file
	crds := map[string]string{}
	namespaces := map[string]bool{}
	for _, p := range catalog.Profiles {
		for _, c := range r.cat.Components {
			if c.Defaults[p] == "unsupported" {
				continue
			}
			for _, rel := range c.Releases {
				data, err := os.ReadFile(filepath.Join(dir, p, c.ID, rel.Name+".yaml"))
				if err != nil {
					return err
				}
				namespaces[rel.Namespace] = true
				// kubectl -n refuses objects that name another namespace.
				byNamespace := map[string]*strings.Builder{}
				for _, doc := range strings.Split(string(data), "\n---") {
					var o struct {
						Kind     string
						Metadata struct{ Name, Namespace string }
					}
					if yaml.Unmarshal([]byte(doc), &o) != nil {
						continue
					}
					switch {
					case o.Kind == "CustomResourceDefinition":
						crds[o.Metadata.Name] = doc
					case o.Kind == "Namespace":
						namespaces[o.Metadata.Name] = true
					case o.Metadata.Namespace != "":
						namespaces[o.Metadata.Namespace] = true
					}
					ns := cmp.Or(o.Metadata.Namespace, rel.Namespace)
					if byNamespace[ns] == nil {
						byNamespace[ns] = &strings.Builder{}
					}
					byNamespace[ns].WriteString(doc + "\n---\n")
				}
				for _, ns := range slices.Sorted(maps.Keys(byNamespace)) {
					files = append(files, file{p + " " + c.ID + "/" + rel.Name, ns, []byte(byNamespace[ns].String()), false})
				}
			}
		}
	}
	for _, c := range r.cat.Components {
		for _, doc := range strings.Split(c.NetworkPolicies, "\n---") {
			var o struct{ Metadata struct{ Namespace string } }
			if strings.TrimSpace(doc) == "" || yaml.Unmarshal([]byte(doc), &o) != nil {
				continue
			}
			files = append(files, file{c.ID + " networkPolicies", o.Metadata.Namespace, []byte(doc), false})
		}
	}
	var all strings.Builder
	for _, name := range slices.Sorted(maps.Keys(crds)) {
		all.WriteString(crds[name] + "\n---\n")
	}
	slog.Info("applying CRDs", "count", len(crds))
	if _, err := catalog.Run([]byte(all.String()), "kubectl", "apply", "--server-side", "--force-conflicts", "-f", "-"); err != nil {
		return err
	}
	if _, err := catalog.Run(nil, "kubectl", "wait", "--for=condition=Established", "crd", "--all", "--timeout=2m"); err != nil {
		return err
	}
	var nsList bytes.Buffer
	for _, ns := range slices.Sorted(maps.Keys(namespaces)) {
		fmt.Fprintf(&nsList, "{\"apiVersion\": \"v1\", \"kind\": \"Namespace\", \"metadata\": {\"name\": %q}}\n", ns)
	}
	if _, err := catalog.Run(nsList.Bytes(), "kubectl", "apply", "-f", "-"); err != nil {
		return err
	}
	for _, t := range r.templates {
		ns := templates.Namespace(t)
		if err := templates.CreateNamespace(t, ns); err != nil {
			return err
		}
		namespaces[ns] = true
		v, err := e2eValues(r.templates, t)
		if err != nil {
			return err
		}
		manifest, err := templates.E2EManifest(t, v)
		if err != nil {
			return fmt.Errorf("templates/%s: %w", t.ID, err)
		}
		secrets, err := templates.GenerateInputs(t, ns)
		if err != nil {
			return err
		}
		docs := [][]byte{[]byte(v.Setup), manifest}
		if slices.ContainsFunc(t.Inputs, func(in templates.Input) bool { return in.Generate != "" }) {
			docs = append(docs, secrets)
		}
		for _, d := range docs {
			if len(bytes.TrimSpace(d)) > 0 {
				files = append(files, file{"templates/" + t.ID, ns, d, true})
			}
		}
	}
	// The API server has no controller manager to create each namespace's
	// default ServiceAccount, which Pods without one use.
	var accounts bytes.Buffer
	for _, ns := range slices.Sorted(maps.Keys(namespaces)) {
		fmt.Fprintf(&accounts, "{\"apiVersion\": \"v1\", \"kind\": \"ServiceAccount\", \"metadata\": {\"name\": \"default\", \"namespace\": %q}}\n", ns)
	}
	if _, err := catalog.Run(accounts.Bytes(), "kubectl", "apply", "-f", "-"); err != nil {
		return err
	}
	errs := make([]error, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, f := range files {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			args := []string{"apply", "--server-side", "--force-conflicts", "--dry-run=server",
				"--field-manager=catalog", "-n", f.namespace, "-f", "-"}
			if f.strict {
				// Pod Security warns on workloads whose pods its enforce level rejects.
				args = append(args, "--warnings-as-errors")
			}
			_, err := catalog.Run(f.data, "kubectl", args...)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", f.subject, err)
			}
		})
	}
	wg.Wait()
	slog.Info("dry run done", "manifests", len(files))
	return errors.Join(errs...)
}

// l1 installs and smoke tests each group in its own single-node cluster.
// Without ids it tests what changed since o.since.
func l1(r *repo, cfg config, o tierOptions, args []string) error {
	sel, err := selection(r, o.since, args)
	if err != nil {
		return err
	}
	groups, err := l1Groups(r, sel)
	if err != nil {
		return err
	}
	var rows []row
	for i, g := range groups {
		slog.Info("l1 group", "n", i+1, "of", len(groups), "tests", strings.Join(g.IDs, " "))
		rows = append(rows, l1Group(r, cfg, o, g)...)
	}
	return report("l1", rows)
}

func l1Group(r *repo, cfg config, o tierOptions, g *group) []row {
	var needs []*catalog.Component
	for _, id := range g.IDs {
		order, _ := installOrder(r, id, templateProfile)
		needs = append(needs, order...)
	}
	var rows []row
	if !o.current {
		start := time.Now()
		c, err := createCluster(templateProfile, cfg.E2E.Version, needs)
		setup := row{subject: "cluster for " + g.IDs[0] + " and others", err: err}
		setup.timings.Since("cluster", start)
		rows = append(rows, setup)
		if err != nil {
			return rows
		}
		if !o.keep {
			defer c.delete()
		}
	}
	t, err := newTester(r.cat, templateProfile)
	if err != nil {
		return append(rows, row{subject: g.IDs[0], err: err})
	}
	t.Share()
	for _, id := range g.IDs {
		t.Timings = nil
		var err error
		if strings.Contains(id, "/") {
			slog.Info("testing", "component", id)
			err = t.Quick(id, g.deps[id])
		} else {
			_, err = runTemplate(r, t, id, false)
		}
		rows = append(rows, row{subject: id, timings: t.Timings, err: err})
	}
	return rows
}

// runTemplate runs a template's e2e and records the time after its
// dependencies as the template phase. If measure is set, it returns the idle
// usage of the template.
func runTemplate(r *repo, t *catalog.Tester, id string, measure bool) (*templates.Usage, error) {
	slog.Info("testing", "template", id)
	tmpl, err := templates.Get(r.templates, id)
	if err != nil {
		return nil, err
	}
	values, err := e2eValues(r.templates, tmpl)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	usage, err := templates.E2E(t, tmpl, values, measure)
	took := time.Since(start)
	for _, p := range t.Timings {
		took -= p.Took
	}
	t.Timings = append(t.Timings, catalog.Phase{Name: "template", Took: took.Round(time.Second)})
	return usage, err
}

// testTemplate runs the full template test on the current cluster and writes
// the measurement if write is set.
func testTemplate(r *repo, t *catalog.Tester, id string, write bool) (status string, err error) {
	key, err := currentEnvironment(t.Profile)
	if err != nil {
		return "", err
	}
	usage, err := runTemplate(r, t, id, true)
	if err != nil {
		return "", err
	}
	if write {
		tmpl, _ := templates.Get(r.templates, id)
		values, err := e2eValues(r.templates, tmpl)
		if err != nil {
			return "", err
		}
		manifest, err := templates.E2EManifest(tmpl, values)
		if err != nil {
			return "", err
		}
		if err := writeTemplateMeasurement(id, key, *usage, imagesOf(manifest)); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("PASS %4.0f MiB %3.0fm CPU", usage.IdleMemoryMiB, usage.IdleCPUm), nil
}

// l2 tests each component and template in a fresh cluster with the full
// checks.
func l2(r *repo, cfg config, o tierOptions, args []string) error {
	if !slices.Contains(l2Profiles(), o.profile) {
		return fmt.Errorf("unknown -profile %q; use %s", o.profile, strings.Join(l2Profiles(), ", "))
	}
	sel, err := selection(r, o.since, args)
	if err != nil {
		return err
	}
	subs, err := profileSubjects(r, o.profile, sel)
	if err != nil {
		return err
	}
	for _, id := range args {
		if !slices.ContainsFunc(subs, func(s subject) bool { return s.id == id }) {
			return fmt.Errorf("%s is not tested on %s", id, o.profile)
		}
	}
	values := o.profile
	if slices.Contains(hostProfiles, values) {
		values = templateProfile
	}
	t, err := newTester(r.cat, values)
	if err != nil {
		return err
	}
	if o.previous != "" {
		if t.Previous, err = readPrevious(o.previous); err != nil {
			return err
		}
	}
	var rows []row
	for _, s := range subs {
		t.Timings = nil
		var c *cluster
		if !o.current {
			start := time.Now()
			if slices.Contains(hostProfiles, o.profile) {
				c, err = createHost(cfg, o.profile, s.order)
			} else {
				c, err = createCluster(o.profile, cfg.E2E.Version, s.order)
			}
			t.Timings.Since("cluster", start)
			if err != nil {
				if o.write && strings.Contains(s.id, "/") {
					c, _ := r.cat.Get(s.id)
					err = errors.Join(err, markFailed(cfg, o.profile, runtime.GOARCH, c, ""))
				}
				rows = append(rows, row{subject: s.id, timings: t.Timings, err: err})
				continue
			}
		}
		rw := row{subject: s.id}
		if strings.Contains(s.id, "/") {
			rw.status, rw.err = testComponent(t, cfg, o.profile, s.id, o.write)
		} else {
			rw.status, rw.err = testTemplate(r, t, s.id, o.write)
		}
		rw.timings = t.Timings
		rows = append(rows, rw)
		if c != nil && !o.keep {
			c.delete()
		}
	}
	return report("l2 "+o.profile, rows)
}

// testComponent runs the full component test on the current cluster. If
// write is set, it writes the measurement under profile, or marks the
// environment failed.
func testComponent(t *catalog.Tester, cfg config, profile, id string, write bool) (status string, err error) {
	c, _ := t.Catalog.Get(id)
	key, err := currentEnvironment(profile)
	if err != nil {
		if write {
			err = errors.Join(err, markFailed(cfg, profile, runtime.GOARCH, c, ""))
		}
		return "", err
	}
	slog.Info("testing", "component", id, "env", key)
	res := t.Test(id)
	if res.Err != nil {
		if write {
			return "", errors.Join(res.Err, markFailed(cfg, profile, runtime.GOARCH, c, key))
		}
		return "", res.Err
	}
	status = fmt.Sprintf("PASS %4.0f MiB %3.0fm CPU", res.IdleMemoryMiB, res.IdleCPUm)
	if res.Unmeasured {
		status = "PASS (installed without credentials; not measured)"
	} else if write {
		if err := writeMeasurement(c, key, res); err != nil {
			return "", err
		}
	}
	if res.UpgradedFrom != "" {
		status += " (upgraded from " + res.UpgradedFrom + ")"
	}
	return status, nil
}

// currentEnvironment returns the environment key of the current cluster.
func currentEnvironment(profile string) (string, error) {
	out, err := catalog.Run(nil, "kubectl", "version", "-o", "json")
	if err != nil {
		return "", err
	}
	return environmentKey(profile, out)
}
