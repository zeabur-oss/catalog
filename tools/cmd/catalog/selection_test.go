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
	return r
}

// record adds a run of id in env, of its current files unless digest is set.
func record(t *testing.T, r *repo, id string, env Environment, digest string) {
	t.Helper()
	path, src := measurementPath(id), ""
	if c, ok := r.cat.Get(id); ok {
		src = c.Path
	} else {
		path, src = templateMeasurementPath(id), filepath.Join("templates", id)
	}
	if digest == "" {
		var err error
		if digest, err = sourceDigest(src); err != nil {
			t.Fatal(err)
		}
	}
	env.SourceDigest = digest
	if env.Result == "" {
		env.Result = resultPassed
	}
	if env.MeasuredAt == "" {
		env.MeasuredAt = "2026-11-07"
	}
	m := readMeasurement(path)
	m.put(env)
	if err := writeMeasurementFile(path, m); err != nil {
		t.Fatal(err)
	}
}

// recordAll records a passing run of the current files of every component and
// template on single-node, amd64 and arm64.
func recordAll(t *testing.T, r *repo) {
	t.Helper()
	for _, id := range everything {
		for _, arch := range []string{"amd64", "arm64"} {
			record(t, r, id, runEnvironment(templateProfile, arch), "")
		}
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
		t.Errorf("stale %v, want nothing after a passing run of every current file", got)
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

func TestFailedLastRunIsStale(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	failed := runEnvironment(templateProfile, "amd64")
	failed.Result, failed.MeasuredAt, failed.KubernetesVersion = resultFailed, "2026-11-08", "v1.36.5"
	record(t, r, "local-storage", failed, "")
	want := []string{"beta", "gamma", "local-storage"}
	if got := staleOn(t, r, templateProfile, "amd64"); !slices.Equal(got, want) {
		t.Errorf("stale %v, want %v", got, want)
	}
	passed := runEnvironment(templateProfile, "amd64")
	passed.MeasuredAt, passed.KubernetesVersion = "2026-11-09", "v1.36.6"
	record(t, r, "local-storage", passed, "")
	if got := staleOn(t, r, templateProfile, "amd64"); len(got) > 0 {
		t.Errorf("stale %v, want nothing once a later run passed", got)
	}
}

func TestStaleIsPerArchitecture(t *testing.T) {
	r := staleRepo(t)
	recordAll(t, r)
	record(t, r, "traefik", runEnvironment(templateProfile, "arm64"), "h1:older")
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
		record(t, r, id, runEnvironment("k3s", "amd64"), "")
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
