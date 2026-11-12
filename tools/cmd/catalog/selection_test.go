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
	if err := os.WriteFile(filepath.Join(dir, "config", "e2e.yaml"), []byte("templates: {}\nsmokeRepresentative: [traefik, local-storage, beta]\n"), 0o644); err != nil {
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

func TestSharedChangePerTier(t *testing.T) {
	r := scopeRepo(t)
	representative := []string{"beta", "local-storage", "traefik"}
	for _, c := range []struct {
		name  string
		files []string
		smoke []string
		full  []string
	}{
		{"tool", []string{"tools/cmd/catalog/tier.go"}, representative, everything},
		{"go.sum", []string{"go.sum"}, representative, everything},
		{"flake", []string{"flake.lock"}, representative, everything},
		{"other config", []string{"config/e2e.yaml"}, representative, everything},
		{"workflow", []string{".github/workflows/ci.yml"}, representative, everything},
		{"kubernetes versions", []string{"config/kubernetes.yaml"}, everything, everything},
		{"documentation", []string{"README.md"}, nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			for tier, want := range map[string][]string{"smoke": c.smoke, "full": c.full} {
				p, err := filesPick(r, tier, c.files)
				if err != nil {
					t.Fatal(err)
				}
				if got := selected(t, r, p); !slices.Equal(got, want) {
					t.Errorf("%s selects %v, want %v", tier, got, want)
				}
			}
		})
	}
}

func TestChangeSelectsWhatInstallsIt(t *testing.T) {
	r := scopeRepo(t)
	for _, tier := range []string{"smoke", "full"} {
		p, err := filesPick(r, tier, []string{"components/traefik.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"beta", "gamma", "metrics-server", "traefik"}
		if got := selected(t, r, p); !slices.Equal(got, want) {
			t.Errorf("%s selects %v, want the component and what installs it: %v", tier, got, want)
		}
	}
}

func TestTemplateChangeSelectsTheTemplate(t *testing.T) {
	r := scopeRepo(t)
	p, err := filesPick(r, "smoke", []string{"templates/gamma/manifests.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if got := selected(t, r, p); !slices.Equal(got, []string{"gamma"}) {
		t.Errorf("selected %v, want only gamma", got)
	}
}

func TestSharedChangeKeepsOwnChange(t *testing.T) {
	r := scopeRepo(t)
	p, err := filesPick(r, "smoke", []string{"config/e2e.yaml", "components/postgres.yaml"})
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

func TestIDsSelectOnlyThem(t *testing.T) {
	r := scopeRepo(t)
	p, err := choose(r, "full", reason{ids: []string{"traefik", "alpha"}})
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
		p, err := choose(r, tier, reason{})
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
	if _, err := choose(r, "full", reason{ids: []string{"ingress/nginx"}}); err == nil {
		t.Error("an unknown ID is accepted")
	}
}

func TestOneReasonAtATime(t *testing.T) {
	r := scopeRepo(t)
	if _, err := choose(r, "smoke", reason{ids: []string{"traefik"}, since: "HEAD"}); err == nil {
		t.Error("IDs and -since together are accepted")
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
