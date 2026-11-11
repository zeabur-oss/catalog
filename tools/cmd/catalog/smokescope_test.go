package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// scopeRepo is a catalog where metrics/server needs ingress/traefik, three
// templates need different parts of it, and the representative list is
// traefik, storage/local and beta.
func scopeRepo(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "e2e.yaml"), []byte("templates: {}\nsmokeRepresentative: [ingress/traefik, storage/local, beta]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	comp := func(id, capability, def string, requires ...string) *catalog.Component {
		c := &catalog.Component{ID: id, Capability: capability, Path: "components/" + id + ".yaml",
			Defaults: map[string]string{"single-node": def}}
		for _, r := range requires {
			c.Requires = append(c.Requires, catalog.Ref{Component: r})
		}
		return c
	}
	tmpl := func(id string, requires ...string) *templates.Template {
		tt := &templates.Template{ID: id}
		for _, r := range requires {
			tt.Requires = append(tt.Requires, catalog.Ref{Component: r})
		}
		return tt
	}
	return &repo{
		cat: &catalog.Catalog{Components: []*catalog.Component{
			comp("ingress/traefik", "ingress", "recommended"),
			comp("storage/local", "storage", "recommended"),
			comp("metrics/server", "metrics", "optional", "ingress/traefik"),
			comp("db/postgres", "db", "optional"),
		}},
		templates: []*templates.Template{
			tmpl("alpha", "db/postgres"),
			tmpl("beta", "ingress/traefik", "storage/local"),
			tmpl("gamma", "ingress/traefik", "storage/local"),
		},
	}
}

func selected(t *testing.T, r *repo, s scope) []string {
	t.Helper()
	groups, err := smokeGroups(r, s.selects)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, g := range groups {
		ids = append(ids, g.IDs...)
	}
	slices.Sort(ids)
	return ids
}

func TestSmokeScopeSharedChange(t *testing.T) {
	r := scopeRepo(t)
	want := []string{"beta", "ingress/traefik", "storage/local"}
	for _, c := range []struct {
		name  string
		files []string
		want  []string
	}{
		{"tool", []string{"tools/cmd/catalog/tier.go"}, want},
		{"go.sum", []string{"go.sum"}, want},
		{"flake", []string{"flake.lock"}, want},
		{"other config", []string{"config/e2e.yaml"}, want},
		{"workflow", []string{".github/workflows/ci.yml"}, want},
		{"documentation", []string{"README.md"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, err := smokeScope(r, c.files)
			if err != nil {
				t.Fatal(err)
			}
			if got := selected(t, r, s); !slices.Equal(got, c.want) {
				t.Errorf("selected %v, want %v", got, c.want)
			}
		})
	}
}

func TestSmokeScopeKubernetesVersions(t *testing.T) {
	r := scopeRepo(t)
	s, err := smokeScope(r, []string{"config/kubernetes.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	got := selected(t, r, s)
	for _, id := range []string{"alpha", "db/postgres", "metrics/server", "gamma"} {
		if !slices.Contains(got, id) {
			t.Errorf("%s is not selected in %v", id, got)
		}
	}
}

func TestSmokeScopeComponentChange(t *testing.T) {
	r := scopeRepo(t)
	s, err := smokeScope(r, []string{"components/ingress/traefik.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"beta", "gamma", "ingress/traefik", "metrics/server"}
	if got := selected(t, r, s); !slices.Equal(got, want) {
		t.Errorf("selected %v, want the component and what installs it: %v", got, want)
	}
}

func TestSharedChangeKeepsOwnChange(t *testing.T) {
	r := scopeRepo(t)
	s, err := smokeScope(r, []string{"config/e2e.yaml", "components/db/postgres.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	got := selected(t, r, s)
	for _, id := range []string{"beta", "db/postgres", "alpha"} {
		if !slices.Contains(got, id) {
			t.Errorf("%s is not selected in %v", id, got)
		}
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

func TestArmSelector(t *testing.T) {
	r := scopeRepo(t)
	for _, c := range []struct {
		name  string
		files []string
		want  []string
	}{
		{"shared file", []string{"tools/cmd/catalog/tier.go"}, []string{"ingress/traefik", "storage/local"}},
		{"representative component", []string{"components/ingress/traefik.yaml"}, []string{"ingress/traefik"}},
		{"other component", []string{"components/db/postgres.yaml"}, nil},
		{"template", []string{"templates/beta/template.yaml"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, err := smokeScope(r, c.files)
			if err != nil {
				t.Fatal(err)
			}
			arm, err := armSelector(r, s.selects)
			if err != nil {
				t.Fatal(err)
			}
			groups, err := smokeGroups(r, arm)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, g := range groups {
				got = append(got, g.IDs...)
			}
			slices.Sort(got)
			if !slices.Equal(got, c.want) {
				t.Errorf("arm64 tests %v, want %v", got, c.want)
			}
		})
	}
}
