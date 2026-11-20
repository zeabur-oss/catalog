package templates_test

import (
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

func TestReadmeNamesTheSecretOfEachGeneratedInput(t *testing.T) {
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
	l := tmpl.Metadata.Locales["zh-TW"]
	l.Readme = "部署後就能使用。"
	tmpl.Metadata.Locales["zh-TW"] = l
	err = templates.Check(root, cat, []*templates.Template{tmpl})
	if err == nil || !strings.Contains(err.Error(), "metadata.locales.zh-TW.readme does not name Secret browserless-secrets") {
		t.Errorf("got %v", err)
	}
}
