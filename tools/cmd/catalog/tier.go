package main

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// Tiers. The check tier of every pull request is scripts/ci/check.sh, which runs
// zeabur-catalog check, render and dry-run. smoke runs in
// the merge queue: it installs and smoke tests what changed, what it needs and
// what installs it, single-node amd64, in groups that share their
// dependencies. full runs what is stale after a push to main and weekly: every
// profile and architecture in a fresh cluster each, with
// idle usage, uninstall, and residue checks. With -cluster current, smoke and full use the current cluster
// instead of kind.
var tiers = []string{"smoke", "full"}

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
	slices.SortStableFunc(items, func(a, b candidate) int { return len(b.deps) - len(a.deps) })
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
		best.IDs = append(best.IDs, it.ID())
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
// SeaweedFS component when its e2e.yaml sets s3.
func templateRefs(r *repo, t *templates.Template) ([]catalog.Ref, error) {
	values, err := e2eValues(r, t)
	if err != nil {
		return nil, err
	}
	refs := slices.Clone(t.Requires)
	if values.S3 {
		refs = append(refs, catalog.Ref{Component: templates.S3Component})
	}
	return refs, nil
}

// candidate is a subject that a run may test.
type candidate struct {
	subject
	order []*catalog.Component // what the test installs, in order
	deps  []string             // the IDs in order, without the subject
	cost  int                  // see groupCost
}

// subjects returns every component that profile supports and, on
// single-node, every template, that p selects.
func subjects(r *repo, profile string, p pick) ([]candidate, error) {
	var out []candidate
	for _, c := range r.cat.Components {
		if c.Defaults[profile] == "unsupported" {
			continue
		}
		order, err := r.cat.InstallOrder(catalog.ByID(c.ID), profile)
		if err != nil {
			return nil, err
		}
		if p.has(c.ID, order) {
			out = append(out, candidate{componentSubject{r, c}, order, ids(order[:len(order)-1]), 1})
		}
	}
	if profile != templateProfile {
		return out, nil
	}
	for _, t := range r.templates {
		sub := templateSubject{r, t}
		order, err := sub.Order(profile)
		if err != nil {
			return nil, err
		}
		if p.has(t.ID, order) {
			out = append(out, candidate{sub, order, ids(order), len(t.Services) + len(t.Requires)})
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
func profileSubjects(r *repo, profile string, p pick) ([]candidate, error) {
	if !slices.Contains(hostProfiles, profile) {
		return subjects(r, profile, p)
	}
	subs, err := subjects(r, templateProfile, p)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(subs, func(s candidate) bool {
		c, ok := r.cat.Get(s.ID())
		return !ok || !hostRuns(profile, c)
	}), nil
}

// installOrder returns what testing id installs.
func installOrder(r *repo, id, profile string) ([]*catalog.Component, error) {
	s, ok := r.subjectOf(id)
	if !ok {
		return nil, fmt.Errorf("unknown component or template %q", id)
	}
	return s.Order(profile)
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
	case "smoke":
		return runSmoke(r, cfg, o)
	case "full":
		return runFull(r, cfg, o)
	}
	return fmt.Errorf("unknown -tier %q; use %s", tier, strings.Join(tiers, ", "))
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
		c, err := createCluster(templateProfile, cfg.E2E.Version, r.e2e.NodeSetup, needs)
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
	t, err := newTester(r, templateProfile)
	if err != nil {
		return append(rows, row{subject: g.IDs[0], err: err})
	}
	t.Share()
	for _, id := range g.IDs {
		t.Timings = nil
		sub, ok := r.subjectOf(id)
		var err error
		if ok {
			_, err = sub.Run(t, runSpec{smoke: true, keep: g.deps[id]})
		} else {
			err = fmt.Errorf("unknown component or template %q", id)
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
	values, err := e2eValues(r, tmpl)
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
		if !slices.ContainsFunc(subs, func(s candidate) bool { return s.ID() == id }) {
			return fmt.Errorf("%s is not tested on %s", id, o.profile)
		}
	}
	values := o.profile
	if slices.Contains(hostProfiles, values) {
		values = templateProfile
	}
	t, err := newTester(r, values)
	if err != nil {
		return err
	}
	var rows []row
	for _, s := range subs {
		t.Timings = nil
		in, err := inputs(r, cfg, s.ID(), o.profile, runtime.GOARCH)
		if err != nil {
			return err
		}
		var c *cluster
		var setupErr error
		if !o.current {
			start := time.Now()
			if slices.Contains(hostProfiles, o.profile) {
				c, err = createHost(cfg, o.profile, r.e2e.NodeSetup, s.order)
			} else {
				c, err = createCluster(o.profile, cfg.E2E.Version, r.e2e.NodeSetup, s.order)
			}
			t.Timings.Since("cluster", start)
			setupErr = err
		}
		rw := row{subject: s.ID()}
		rw.status, rw.err = s.Run(t, runSpec{cfg: cfg, profile: o.profile, in: in, write: o.write, setupErr: setupErr})
		rw.timings = t.Timings
		rows = append(rows, rw)
		if c != nil && !o.keep {
			c.delete()
		}
	}
	return report("full "+o.profile, rows)
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
