package templates_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestTemplateIDMustNotBeAComponentID(t *testing.T) {
	const root = "../../.."
	cat, err := catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	all, err := templates.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	all[0].ID = "traefik"
	err = templates.Check(root, cat, all[:1])
	if err == nil || !strings.Contains(err.Error(), "components/traefik.yaml has the same ID") {
		t.Errorf("got %v", err)
	}
}

func TestRequiresCertificatesByComponentOrCapability(t *testing.T) {
	cat := &catalog.Catalog{Components: []*catalog.Component{
		{ID: "cert-manager", Provides: &catalog.Provides{Capability: "certificates"}},
		{ID: "traefik", Provides: &catalog.Provides{Capability: "ingress"}},
	}}
	for _, c := range []struct {
		ref  catalog.Ref
		want bool
	}{
		{catalog.Ref{Component: "cert-manager"}, true},
		{catalog.Ref{Capability: "certificates"}, true},
		{catalog.Ref{Component: "traefik"}, false},
	} {
		tmpl := &templates.Template{Requires: []catalog.Ref{c.ref}}
		if got := tmpl.RequiresCertificates(cat); got != c.want {
			t.Errorf("%+v: got %v, want %v", c.ref, got, c.want)
		}
	}
}

func TestReadmesHaveTheSectionsAndNameGeneratedSecrets(t *testing.T) {
	const root = "../../.."
	cat, err := catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	all, err := templates.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := templates.Get(all, "browserless")
	if err != nil {
		t.Fatal(err)
	}
	if err := templates.Check(root, cat, []*templates.Template{tmpl}); err != nil {
		t.Fatalf("as written: %v", err)
	}
	tmpl.Dir = filepath.Join(t.TempDir(), "browserless")
	if err := os.CopyFS(tmpl.Dir, os.DirFS(filepath.Join(root, "templates", "browserless"))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpl.Dir, "README.md"), []byte("# Browserless\n\n## Introduction\n\nHeadless Chrome.\n\n## Usage\n\nConnect.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := "## 簡介\n\n無頭 Chrome。\n\n## 開始使用\n\n部署後就能使用。\n\n## 設定\n\n權杖在 Secret browserless-secrets。\n"
	if err := os.WriteFile(filepath.Join(tmpl.Dir, "README.zh-TW.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	err = templates.Check(root, cat, []*templates.Template{tmpl})
	for _, want := range []string{
		`README.md: level-1 heading "# Browserless"`,
		`README.md: heading "## Usage"`,
		"README.md: no ## Getting started",
		"README.zh-TW.md: 開始使用 does not name Secret browserless-secrets",
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}
