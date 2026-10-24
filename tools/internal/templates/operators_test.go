package templates_test

import (
	"strings"
	"testing"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestCNPGCannotBorrowUnrelatedProviderTrust(t *testing.T) {
	r := &templates.Template{Requires: []string{"ingress/traefik"}}
	r.Security.Risks = []map[string]string{{"risk": "Operator generated pods and SA", "mitigation": "Pinned catalog review"}}
	b := safeManifest + "---\napiVersion: postgresql.cnpg.io/v1\nkind: Cluster\nmetadata: {name: database}\nspec:\n  imageName: ghcr.io/example/postgres@sha256:" + strings.Repeat("a", 64) + "\n  seccompProfile: {type: RuntimeDefault}\n  resources:\n    requests: {cpu: 10m, memory: 16Mi}\n    limits: {cpu: 100m, memory: 32Mi}\n"
	if err := templates.Safety(r, []byte(b), "test"); err == nil {
		t.Fatal("CNPG allowed without its exact catalog provider")
	}
	r.Requires = []string{"databases/cloudnative-pg"}
	if err := templates.Safety(r, []byte(b), "test"); err != nil {
		t.Fatal(err)
	}
}
func TestMiddlewareAdapterOnlyAllowsExplicitIPAllowList(t *testing.T) {
	r := &templates.Template{Requires: []string{"ingress/traefik"}}
	prefix := safeManifest + "---\napiVersion: traefik.io/v1alpha1\nkind: Middleware\nmetadata: {name: admin}\nspec:\n"
	if err := templates.Safety(r, []byte(prefix+"  ipAllowList: {sourceRange: [192.0.2.0/24]}\n"), "test"); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []string{"  ipAllowList: {sourceRange: [0.0.0.0/0]}\n", "  forwardAuth: {address: 'http://another-namespace'}\n", "  ipAllowList: {sourceRange: [192.0.2.0/24]}\n  stripPrefix: {prefixes: [/]}\n"} {
		if templates.Safety(r, []byte(prefix+spec), "test") == nil {
			t.Fatal("unsupported Middleware behavior accepted")
		}
	}
}

func TestIngressRequiresDeclaredRuntimeProviders(t *testing.T) {
	manifest := safeManifest + `---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app
  annotations: {cert-manager.io/cluster-issuer: issuer}
spec:
  ingressClassName: traefik
`
	for _, tc := range []struct {
		name      string
		providers []string
		reject    bool
	}{
		{name: "complete", providers: []string{"ingress/traefik", "certificates/cert-manager"}},
		{name: "missing-certificates", providers: []string{"ingress/traefik"}, reject: true},
		{name: "missing-ingress", providers: []string{"certificates/cert-manager"}, reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &templates.Template{Requires: tc.providers}
			err := templates.Safety(r, []byte(manifest), "test")
			if (err != nil) != tc.reject {
				t.Fatalf("rejected = %v, want %v: %v", err != nil, tc.reject, err)
			}
		})
	}
}
