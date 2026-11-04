package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// scopeRepo is a catalog where ingress/traefik and storage/local are
// recommended, metrics/server is optional and needs traefik, and three
// templates need different parts of them.
func scopeRepo(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "e2e.yaml"), []byte("templates: {}\n"), 0o644); err != nil {
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

func neverChecked(t *testing.T) toolsChanged {
	return func(bool, bool) (bool, error) {
		t.Error("compared the test tools")
		return false, nil
	}
}

func TestSmokeScopeSharedChange(t *testing.T) {
	r := scopeRepo(t)
	want := []string{"beta", "ingress/traefik", "storage/local"}
	for _, c := range []struct {
		name  string
		files []string
		check toolsChanged
		want  []string
	}{
		{"tool changed", []string{"tools/cmd/catalog/tier.go"}, func(goTool, nixTools bool) (bool, error) { return goTool && !nixTools, nil }, want},
		{"flake changed the tools", []string{"flake.lock"}, func(goTool, nixTools bool) (bool, error) { return nixTools && !goTool, nil }, want},
		{"tool unchanged", []string{"tools/cmd/catalog/tier_test.go", "go.sum"}, func(bool, bool) (bool, error) { return false, nil }, nil},
		{"cannot compare", []string{"go.mod"}, func(bool, bool) (bool, error) { return false, os.ErrNotExist }, want},
		{"other config", []string{"config/e2e.yaml"}, nil, want},
		{"workflow", []string{".github/workflows/ci.yml"}, nil, want},
		{"documentation", []string{"README.md"}, nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			check := c.check
			if check == nil {
				check = neverChecked(t)
			}
			s, err := smokeScope(r, c.files, check)
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
	s, err := smokeScope(r, []string{"config/kubernetes.yaml"}, neverChecked(t))
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
	s, err := smokeScope(r, []string{"components/ingress/traefik.yaml"}, neverChecked(t))
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
	s, err := smokeScope(r, []string{"config/e2e.yaml", "components/db/postgres.yaml"}, neverChecked(t))
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

func TestRepresentativeTemplate(t *testing.T) {
	r := scopeRepo(t)
	ids, err := representative(r)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ids, "beta") || slices.Contains(ids, "gamma") || slices.Contains(ids, "alpha") {
		t.Errorf("representative = %v, want beta, the first template that installs the most recommended components", ids)
	}
}
