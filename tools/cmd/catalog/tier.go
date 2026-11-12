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
	"go.yaml.in/yaml/v4"
)

// Tiers. check runs on every pull request: static checks, then a server-side
// dry run on a local API server with only the CRDs and no nodes, which also
// fails when the API server refuses an upgrade from the base or a profile switch. smoke runs in
// the merge queue: it installs and smoke tests what changed, what it needs and
// what installs it, single-node amd64, in groups that share their
// dependencies. full runs weekly, before a release and, for what is stale, after
// a push to main: every profile and architecture in a fresh cluster each, with
// idle usage, uninstall, and residue checks. With -cluster current, smoke and full use the current cluster
// instead of kind.
var tiers = []string{"check", "smoke", "full"}

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

// group is what one smoke cluster tests, in order. Its members share dependencies.
type group struct {
	IDs  []string
	deps map[string]bool
	cost int
}

// groupCost caps the cost of one smoke group. A component costs 1, and a template
// 1 per service and per component it requires. On a 4-CPU VM, one unit took
// about 40 seconds.
const groupCost = 20

// smokeGroups returns the single-node subjects that p selects, in groups that
// share dependencies. The grouping is deterministic.
func smokeGroups(r *repo, p pick) ([]*group, error) {
	items, err := subjects(r, templateProfile, p)
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
func templateRefs(r *repo, t *templates.Template) ([]catalog.Ref, error) {
	values, err := e2eValues(r.templates, t)
	if err != nil {
		return nil, err
	}
	refs := slices.Clone(t.Requires)
	if slices.ContainsFunc(slices.Collect(maps.Values(values.Inputs)), func(v string) bool { return strings.Contains(v, ".S3") }) {
		refs = append(refs, catalog.Ref{Component: templates.S3Component})
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
// single-node, every template, that p selects.
func subjects(r *repo, profile string, p pick) ([]subject, error) {
	var out []subject
	for _, c := range r.cat.Components {
		if c.Defaults[profile] == "unsupported" {
			continue
		}
		order, err := r.cat.InstallOrder(catalog.ByID(c.ID), profile)
		if err != nil {
			return nil, err
		}
		if p.has(c.ID, order) {
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
		if p.has(t.ID, order) {
			out = append(out, subject{t.ID, order, ids(order), len(t.Services) + len(t.Requires)})
		}
	}
	return out, nil
}

// kubesoloProfile is the host profile that installs KubeSolo.
const kubesoloProfile = "kubesolo"

// fullProfiles returns the catalog profiles, then the host profiles.
func fullProfiles() []string { return append(slices.Clone(catalog.Profiles), hostProfiles...) }

// profileSubjects is subjects for a catalog profile or a host profile. A host
// profile tests components only.
func profileSubjects(r *repo, profile string, p pick) ([]subject, error) {
	if !slices.Contains(hostProfiles, profile) {
		return subjects(r, profile, p)
	}
	subs, err := subjects(r, templateProfile, p)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(subs, func(s subject) bool {
		c, ok := r.cat.Get(s.id)
		return !ok || !hostRuns(profile, c)
	}), nil
}

// isComponent reports whether id is a component's. Every other ID is a
// template's; templates.Check keeps the two apart.
func (r *repo) isComponent(id string) bool {
	_, ok := r.cat.Get(id)
	return ok
}

// installOrder returns what testing id installs.
func installOrder(r *repo, id, profile string) ([]*catalog.Component, error) {
	if r.isComponent(id) {
		return r.cat.InstallOrder(catalog.ByID(id), profile)
	}
	t, err := templates.Get(r.templates, id)
	if err != nil {
		return nil, fmt.Errorf("unknown component or template %q", id)
	}
	refs, err := templateRefs(r, t)
	if err != nil {
		return nil, err
	}
	return r.cat.InstallOrder(refs, profile)
}

type tierOptions struct {
	profile     string
	why         reason
	write, keep bool
	current     bool // test on the cluster that KUBECONFIG names instead of kind
}

func runTier(r *repo, tier string, o tierOptions) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	switch tier {
	case "check":
		return runCheck(r, cfg, o)
	case "smoke":
		return runSmoke(r, cfg, o)
	case "full":
		return runFull(r, cfg, o)
	}
	return fmt.Errorf("unknown -tier %q; use %s", tier, strings.Join(tiers, ", "))
}

// runCheck validates, renders and lints everything, then sends every rendered
// component and template to a local API server with --dry-run=server, and
// fails when it refuses an upgrade from the pull request's base or a profile
// switch.
func runCheck(r *repo, cfg config, o tierOptions) error {
	var ts catalog.Timings
	step := func(name string, f func() error) error {
		start := time.Now()
		err := f()
		ts.Since(name, start)
		if err != nil {
			report("check", []row{{subject: "check", timings: ts, err: fmt.Errorf("%s: %w", name, err)}})
		}
		return err
	}
	if err := step("validate", func() error {
		_, repErr := representative(r)
		return errors.Join(catalog.Check(".", r.cat), templates.Check(".", r.cat, r.templates), repErr)
	}); err != nil {
		return err
	}
	out := "out/render"
	if err := step("render", func() error { return renderComponents(r.cat, r.cat.Components, out, cfg.Versions) }); err != nil {
		return err
	}
	if err := step("template lint", func() error { return lintTemplates(r, out, cfg.Versions) }); err != nil {
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
	kv, err := serverVersion(api, cfg.Versions)
	if err != nil {
		return err
	}
	err = step("dry run", func() error { return dryRun(r, filepath.Join(out, kv)) })
	if err == nil {
		err = step("refusals", func() error { return checkRefusals(r, filepath.Join(out, kv)) })
	}
	if err == nil {
		err = report("check", []row{{subject: "check", timings: ts}})
	}
	return err
}

// serverVersion returns the version in versions with the API server's minor.
func serverVersion(api *apiServer, versions []string) (string, error) {
	server, err := api.version()
	if err != nil {
		return "", err
	}
	i := slices.IndexFunc(versions, func(v string) bool { return minorVersion(v) == minorVersion(server) })
	if i < 0 {
		return "", fmt.Errorf("the API server is Kubernetes %s; config/kubernetes.yaml renders no version with the same minor", server)
	}
	return versions[i], nil
}

// lintTemplates runs what `catalog template lint` runs on every template,
// rendered with its e2e values: the safety rules, then kubeconform with only
// the CRDs of the components that its e2e installs. render holds the render of
// every component.
func lintTemplates(r *repo, render string, versions []string) error {
	errs := make([]error, len(r.templates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, t := range r.templates {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			errs[i] = lintTemplate(r, t, render, versions)
			if errs[i] != nil {
				errs[i] = fmt.Errorf("templates/%s: %w", t.ID, errs[i])
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

func lintTemplate(r *repo, t *templates.Template, render string, versions []string) error {
	v, err := e2eValues(r.templates, t)
	if err != nil {
		return err
	}
	manifest, err := templates.E2EManifest(t, v)
	if err != nil {
		return err
	}
	order, err := installOrder(r, t.ID, templateProfile)
	if err != nil {
		return err
	}
	schemas, err := orderSchemas(filepath.Join(render, versions[len(versions)-1], templateProfile), order)
	defer os.RemoveAll(schemas)
	if err != nil {
		return err
	}
	return templates.Lint(t, manifest, templates.Namespace(t), schemas, versions)
}

// orderSchemas writes the schemas of the CRDs that the components of order
// render under dir to a new directory.
func orderSchemas(dir string, order []*catalog.Component) (string, error) {
	schemas, err := os.MkdirTemp("", "catalog-schemas-")
	if err != nil {
		return "", err
	}
	for _, c := range order {
		if err := catalog.CRDSchemas(filepath.Join(dir, c.ID), schemas); err != nil {
			return schemas, err
		}
	}
	return schemas, nil
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
				docs := manifestDocs(data, rel.Namespace)
				for _, d := range docs {
					namespaces[d.Namespace] = true
					switch d.Kind {
					case "CustomResourceDefinition":
						crds[d.Name] = d.text
					case "Namespace":
						namespaces[d.Name] = true
					}
				}
				for _, d := range byNamespace(docs) {
					files = append(files, file{p + " " + c.ID + "/" + rel.Name, d.namespace, []byte(d.docs), false})
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
		if slices.ContainsFunc(t.Inputs, func(in templates.Input) bool { return in.Generate != nil }) {
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

// runSmoke installs and smoke tests each group in its own single-node cluster.
func runSmoke(r *repo, cfg config, o tierOptions) error {
	p, err := choose(r, "smoke", o.why, templateProfile, templateArch)
	if err != nil {
		return err
	}
	groups, err := smokeGroups(r, p)
	if err != nil {
		return err
	}
	var rows []row
	for i, g := range groups {
		slog.Info("smoke group", "n", i+1, "of", len(groups), "tests", strings.Join(g.IDs, " "))
		rows = append(rows, smokeGroup(r, cfg, o, g)...)
	}
	return report("smoke", rows)
}

func smokeGroup(r *repo, cfg config, o tierOptions, g *group) []row {
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
		if r.isComponent(id) {
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
func runTemplate(r *repo, t *catalog.Tester, id string, measure bool) ([]catalog.PodUsage, error) {
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
func testTemplate(r *repo, t *catalog.Tester, cfg config, id string, write bool) (status string, err error) {
	env, err := currentEnvironment(cfg, t.Profile)
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
		if err := writeTemplateMeasurement(tmpl, env, usage, imagesOf(manifest)); err != nil {
			return "", err
		}
	}
	return podSummary(usage), nil
}

// runFull tests each component and template in a fresh cluster with the full
// checks.
func runFull(r *repo, cfg config, o tierOptions) error {
	if !slices.Contains(fullProfiles(), o.profile) {
		return fmt.Errorf("unknown -profile %q; use %s", o.profile, strings.Join(fullProfiles(), ", "))
	}
	p, err := choose(r, "full", o.why, o.profile, runtime.GOARCH)
	if err != nil {
		return err
	}
	subs, err := profileSubjects(r, o.profile, p)
	if err != nil {
		return err
	}
	for _, id := range o.why.ids {
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
				if o.write && r.isComponent(s.id) {
					c, _ := r.cat.Get(s.id)
					err = errors.Join(err, markFailed(cfg, o.profile, runtime.GOARCH, c, nil))
				}
				rows = append(rows, row{subject: s.id, timings: t.Timings, err: err})
				continue
			}
		}
		rw := row{subject: s.id}
		if r.isComponent(s.id) {
			rw.status, rw.err = testComponent(t, cfg, o.profile, s.id, o.write)
		} else {
			rw.status, rw.err = testTemplate(r, t, cfg, s.id, o.write)
		}
		rw.timings = t.Timings
		rows = append(rows, rw)
		if c != nil && !o.keep {
			c.delete()
		}
	}
	return report("full "+o.profile, rows)
}

// testComponent runs the full component test on the current cluster. If
// write is set, it writes the measurement under profile, or marks the
// environment failed.
func testComponent(t *catalog.Tester, cfg config, profile, id string, write bool) (status string, err error) {
	c, _ := t.Catalog.Get(id)
	env, err := currentEnvironment(cfg, profile)
	if err != nil {
		if write {
			err = errors.Join(err, markFailed(cfg, profile, runtime.GOARCH, c, nil))
		}
		return "", err
	}
	slog.Info("testing", "component", id, "distribution", env.Distribution, "platform", env.Platform.String(), "kubernetes", env.KubernetesVersion)
	res := t.Test(id)
	if res.Err != nil {
		if write {
			return "", errors.Join(res.Err, markFailed(cfg, profile, runtime.GOARCH, c, &env))
		}
		return "", res.Err
	}
	status = podSummary(res.Pods)
	if res.Unmeasured {
		status = "PASS (installed without credentials; not measured)"
	} else if write {
		if err := writeMeasurement(c, env, res); err != nil {
			return "", err
		}
	}
	return status, nil
}

// currentEnvironment returns the environment of the current cluster.
func currentEnvironment(cfg config, profile string) (Environment, error) {
	out, err := catalog.Run(nil, "kubectl", "version", "-o", "json")
	if err != nil {
		return Environment{}, err
	}
	return environmentOf(cfg, profile, out)
}

// podSummary is the status of a measured test: the workloads it measured.
func podSummary(pods []catalog.PodUsage) string {
	return fmt.Sprintf("PASS %d workloads measured", len(pods))
}
