package templates_test

import (
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

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
