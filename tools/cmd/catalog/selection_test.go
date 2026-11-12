package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// scopeRepo is a catalog where metrics-server needs traefik, three
// templates need different parts of it, and the representative list is
// traefik, local-storage and beta.
func scopeRepo(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "e2e.yaml"), []byte("templates: {}\nsmokeRepresentative: [traefik, local-storage, beta]\nunready: [postgres]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	comp := func(id, capability, def string, requires ...string) *catalog.Component {
		c := &catalog.Component{ID: id, Provides: &catalog.Provides{Capability: capability}, Path: "components/" + id + ".yaml",
			Defaults: map[string]string{"single-node": def}}
		for _, r := range requires {
			c.Requires = append(c.Requires, catalog.Ref{Component: r})
		}
		return c
	}
	tmpl := func(id string, requires ...string) *templates.Template {
		tt := &templates.Template{ID: id, Dir: filepath.Join("templates", id)}
		for _, r := range requires {
			tt.Requires = append(tt.Requires, catalog.Ref{Component: r})
		}
		return tt
	}
	return &repo{
		cat: &catalog.Catalog{Components: []*catalog.Component{
			comp("traefik", "ingress", "recommended"),
			comp("local-storage", "storage", "recommended"),
			comp("metrics-server", "metrics", "optional", "traefik"),
			comp("postgres", "db", "optional"),
		}},
		templates: []*templates.Template{
			tmpl("alpha", "postgres"),
			tmpl("beta", "traefik", "local-storage"),
			tmpl("gamma", "traefik", "local-storage"),
		},
	}
}

// selected returns the single-node IDs that p selects, sorted.
func selected(t *testing.T, r *repo, p pick) []string {
	t.Helper()
	subs, err := subjects(r, templateProfile, p)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range subs {
		ids = append(ids, s.id)
	}
	slices.Sort(ids)
	return ids
}

var everything = []string{"alpha", "beta", "gamma", "local-storage", "metrics-server", "postgres", "traefik"}

func TestSharedChange(t *testing.T) {
	r := scopeRepo(t)
	representative := []string{"beta", "local-storage", "traefik"}
	for _, c := range []struct {
		name  string
		files []string
		want  []string
	}{
		{"tool", []string{"tools/cmd/catalog/tier.go"}, representative},
		{"go.sum", []string{"go.sum"}, representative},
		{"flake", []string{"flake.lock"}, representative},
		{"other config", []string{"config/e2e.yaml"}, representative},
		{"workflow", []string{".github/workflows/ci.yml"}, representative},
		{"kubernetes versions", []string{"config/kubernetes.yaml"}, everything},
		{"documentation", []string{"README.md"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, err := filesPick(r, c.files)
			if err != nil {
				t.Fatal(err)
			}
			if got := selected(t, r, p); !slices.Equal(got, c.want) {
				t.Errorf("selected %v, want %v", got, c.want)
			}
		})
	}
}

func TestChangeSelectsWhatInstallsIt(t *testing.T) {
	r := scopeRepo(t)
	p, err := filesPick(r, []string{"components/traefik.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"beta", "gamma", "metrics-server", "traefik"}
	if got := selected(t, r, p); !slices.Equal(got, want) {
		t.Errorf("selected %v, want the component and what installs it: %v", got, want)
	}
}

func TestTemplateChangeSelectsTheTemplate(t *testing.T) {
	r := scopeRepo(t)
	p, err := filesPick(r, []string{"templates/gamma/manifests.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if got := selected(t, r, p); !slices.Equal(got, []string{"gamma"}) {
		t.Errorf("selected %v, want only gamma", got)
	}
}

func TestSharedChangeKeepsOwnChange(t *testing.T) {
	r := scopeRepo(t)
	p, err := filesPick(r, []string{"config/e2e.yaml", "components/postgres.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	got := selected(t, r, p)
	for _, id := range []string{"beta", "postgres", "alpha"} {
		if !slices.Contains(got, id) {
			t.Errorf("%s is not selected in %v", id, got)
		}
	}
}

func TestFullTierHasNoSince(t *testing.T) {
	r := scopeRepo(t)
	if _, err := choose(r, "full", reason{since: "HEAD"}, templateProfile, "amd64"); err == nil {
		t.Error("the full tier accepts -since")
	}
	if _, err := choose(r, "smoke", reason{stale: true}, templateProfile, "amd64"); err == nil {
		t.Error("the smoke tier accepts -stale")
	}
}

func TestIDsSelectOnlyThem(t *testing.T) {
	r := scopeRepo(t)
	p, err := choose(r, "full", reason{ids: []string{"traefik", "alpha"}}, templateProfile, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if got := selected(t, r, p); !slices.Equal(got, []string{"alpha", "traefik"}) {
		t.Errorf("selected %v, want alpha and traefik without what installs them", got)
	}
}

func TestNoReasonSelectsEverything(t *testing.T) {
	r := scopeRepo(t)
	for _, tier := range []string{"smoke", "full"} {
		p, err := choose(r, tier, reason{}, templateProfile, "amd64")
		if err != nil {
			t.Fatal(err)
		}
		if got := selected(t, r, p); !slices.Equal(got, everything) {
			t.Errorf("%s selects %v, want everything", tier, got)
		}
	}
}

func TestUnknownIDFails(t *testing.T) {
	r := scopeRepo(t)
	if _, err := choose(r, "full", reason{ids: []string{"ingress/nginx"}}, templateProfile, "amd64"); err == nil {
		t.Error("an unknown ID is accepted")
	}
}

func TestOneReasonAtATime(t *testing.T) {
	r := scopeRepo(t)
	if _, err := choose(r, "smoke", reason{ids: []string{"traefik"}, since: "HEAD"}, templateProfile, "amd64"); err == nil {
		t.Error("IDs and -since together are accepted")
	}
	if _, err := choose(r, "full", reason{ids: []string{"traefik"}, stale: true}, templateProfile, "amd64"); err == nil {
		t.Error("IDs and -stale together are accepted")
	}
}

func TestRepresentativeUnknownID(t *testing.T) {
	r := scopeRepo(t)
	if err := os.WriteFile(filepath.Join("config", "e2e.yaml"), []byte("smokeRepresentative: [nope]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := representative(r); err == nil {
		t.Error("representative accepted an unknown ID")
	}
}

// staleRepo is scopeRepo with the files of every component and template, so
// they have a source digest.
func staleRepo(t *testing.T) *repo {
	t.Helper()
	r := scopeRepo(t)
	for _, c := range r.cat.Components {
		if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(c.Path, []byte("id: "+c.ID+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range r.templates {
		if err := os.MkdirAll(tt.Dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tt.Dir, "template.yaml"), []byte("id: "+tt.ID+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pin(t, "v1.36.4", "v1.36.5+k3s1")
	return r
}

// record adds a passing run of id on profile and arch with the current
// inputs, after change if it is set.
func record(t *testing.T, r *repo, id, profile, arch string, change func(*Environment)) {
	t.Helper()
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	env, err := inputs(r, cfg, id, profile, arch)
	if err != nil {
		t.Fatal(err)
	}
	if env.KubernetesVersion == "" {
		env.KubernetesVersion = "v1.36.5"
	}
	env.Result, env.MeasuredAt = resultPassed, "2026-11-07"
	if change != nil {
		change(&env)
	}
	path := measurementPath(id)
	if !r.isComponent(id) {
		path = templateMeasurementPath(id)
	}
	m := readMeasurement(path)
	m.put(env)
	if err := writeMeasurementFile(path, m); err != nil {
		t.Fatal(err)
	}
}

// recordAll records a passing run of the current inputs of every component
// and template on single-node, amd64 and arm64.
func recordAll(t *testing.T, r *repo) {
	t.Helper()
	for _, id := range everything {
		for _, arch := range []string{"amd64", "arm64"} {
			record(t, r, id, templateProfile, arch, nil)
		}
	}
}

// pin writes config/kubernetes.yaml with the kind node and k3s releases.
func pin(t *testing.T, kind, k3s string) {
	t.Helper()
	data := "e2e:\n  version: " + kind + "\nk3s:\n  version: " + k3s + "\n"
	if err := os.WriteFile(filepath.Join("config", "kubernetes.yaml"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func staleOn(t *testing.T, r *repo, profile, arch string) []string {
	t.Helper()
	p, err := choose(r, "full", reason{stale: true}, profile, arch)
	if err != nil {
		t.Fatal(err)
	}
	subs, err := profileSubjects(r, profile, p)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range subs {
		ids = append(ids, s.id)
	}
	slices.Sort(ids)
	return ids
}

func TestFreshSelectsNothing(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	if got := staleOn(t, r, templateProfile, "amd64"); len(got) > 0 {
		t.Errorf("stale %v, want nothing after a passing run of every current input", got)
	}
}

func TestMissingRunIsStale(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	if err := os.Remove(measurementPath("traefik")); err != nil {
		t.Fatal(err)
	}
	want := []string{"beta", "gamma", "metrics-server", "traefik"}
	if got := staleOn(t, r, templateProfile, "amd64"); !slices.Equal(got, want) {
		t.Errorf("stale %v, want traefik and what installs it: %v", got, want)
	}
}

func TestChangedFileIsStale(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	if err := os.WriteFile(filepath.Join("templates", "alpha", "manifests.yaml"), []byte("kind: Service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := staleOn(t, r, templateProfile, "amd64"); !slices.Equal(got, []string{"alpha"}) {
		t.Errorf("stale %v, want only alpha, whose directory changed", got)
	}
}

func TestChangedDependencyIsStale(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	if err := os.WriteFile(filepath.Join("components", "traefik.yaml"), []byte("id: traefik\nport: 80\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record(t, r, "traefik", templateProfile, "amd64", nil)
	want := []string{"beta", "gamma", "metrics-server"}
	if got := staleOn(t, r, templateProfile, "amd64"); !slices.Equal(got, want) {
		t.Errorf("stale %v, want what ran with the old traefik: %v", got, want)
	}
}

func TestRunWithoutDependencyDigestsIsStale(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	record(t, r, "metrics-server", templateProfile, "amd64", func(e *Environment) { e.Dependencies = nil })
	if got := staleOn(t, r, templateProfile, "amd64"); !slices.Equal(got, []string{"metrics-server"}) {
		t.Errorf("stale %v, want metrics-server, whose run has no digest of traefik", got)
	}
}

func TestPinnedReleaseChangeIsStale(t *testing.T) {
	notUnready := []string{"alpha", "beta", "gamma", "local-storage", "metrics-server", "traefik"}
	hostTested := []string{"local-storage", "metrics-server", "traefik"}
	for _, c := range []struct {
		name, kind, k3s string
		kindStale       []string
		k3sStale        []string
	}{
		{"same", "v1.36.4", "v1.36.5+k3s1", nil, nil},
		{"kind node", "v1.37.0", "v1.36.5+k3s1", notUnready, nil},
		{"k3s", "v1.36.4", "v1.37.2+k3s1", nil, hostTested},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := staleRepo(t)
			for _, comp := range r.cat.Components {
				comp.Distributions = []string{"k3s"}
			}
			recordAll(t, r)
			for _, id := range hostTested {
				record(t, r, id, "k3s", "amd64", nil)
			}
			pin(t, c.kind, c.k3s)
			if got := staleOn(t, r, templateProfile, "amd64"); !slices.Equal(got, c.kindStale) {
				t.Errorf("stale on kind %v, want %v", got, c.kindStale)
			}
			if got := staleOn(t, r, "k3s", "amd64"); !slices.Equal(got, c.k3sStale) {
				t.Errorf("stale on k3s %v, want %v", got, c.k3sStale)
			}
		})
	}
}

func TestMergeDropsRunsOfOtherInputs(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	if err := os.WriteFile(filepath.Join("components", "traefik.yaml"), []byte("id: traefik\nport: 80\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A result of the new traefik, from a run older than the one that
	// recorded the rest.
	record(t, r, "traefik", templateProfile, "amd64", func(e *Environment) { e.MeasuredAt = "2026-11-01" })
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := dropOutdated(r, cfg, "measurements"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		id    string
		archs []string
	}{
		{"traefik", []string{"amd64"}},
		{"metrics-server", nil},
		{"beta", nil},
		{"local-storage", []string{"amd64", "arm64"}},
	} {
		path := measurementPath(c.id)
		if !r.isComponent(c.id) {
			path = templateMeasurementPath(c.id)
		}
		var archs []string
		for _, e := range readMeasurement(path).Environments {
			archs = append(archs, e.Platform.Architecture)
		}
		if !slices.Equal(archs, c.archs) {
			t.Errorf("%s keeps runs on %v, want %v", c.id, archs, c.archs)
		}
	}
}

func TestFailedLastRunIsStale(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	record(t, r, "local-storage", templateProfile, "amd64", func(e *Environment) { e.Result = resultFailed })
	want := []string{"beta", "gamma", "local-storage"}
	if got := staleOn(t, r, templateProfile, "amd64"); !slices.Equal(got, want) {
		t.Errorf("stale %v, want %v", got, want)
	}
	record(t, r, "local-storage", templateProfile, "amd64", nil)
	if got := staleOn(t, r, templateProfile, "amd64"); len(got) > 0 {
		t.Errorf("stale %v, want nothing once a later run passed", got)
	}
}

func TestStaleIsPerArchitecture(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	record(t, r, "traefik", templateProfile, "arm64", func(e *Environment) { e.SourceDigest = "h1:older" })
	if got := staleOn(t, r, templateProfile, "amd64"); len(got) > 0 {
		t.Errorf("stale on amd64 %v, want nothing", got)
	}
	if got := staleOn(t, r, templateProfile, "arm64"); !slices.Contains(got, "traefik") {
		t.Errorf("stale on arm64 %v, want traefik", got)
	}
}

func TestStaleIsPerDistribution(t *testing.T) {
	r := staleRepo(t)
	for _, c := range r.cat.Components {
		c.Distributions = []string{"k3s"}
	}
	recordAll(t, r)
	if got := staleOn(t, r, "k3s", "amd64"); !slices.Contains(got, "traefik") {
		t.Errorf("stale on k3s %v, want traefik, which has run on kind only", got)
	}
	for _, id := range []string{"traefik", "local-storage", "metrics-server"} {
		record(t, r, id, "k3s", "amd64", nil)
	}
	if got := staleOn(t, r, "k3s", "amd64"); len(got) > 0 {
		t.Errorf("stale on k3s %v, want nothing", got)
	}
}

func TestUnreadyIsNeverStale(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	if err := os.Remove(measurementPath("postgres")); err != nil {
		t.Fatal(err)
	}
	if got := staleOn(t, r, templateProfile, "amd64"); len(got) > 0 {
		t.Errorf("stale %v, want nothing: an unready component has no measurement to compare", got)
	}
}
