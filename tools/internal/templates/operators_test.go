package templates_test

import (
	"errors"
	"strings"
	"testing"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestCNPGCannotBorrowUnrelatedProviderTrust(t *testing.T) {
	r := &templates.Template{ID: "app", Requires: []string{"ingress/traefik"}}
	r.Security.Risks = []map[string]string{{"risk": "Operator generated pods and SA", "mitigation": "Pinned catalog review"}}
	b := safeManifest + "---\napiVersion: postgresql.cnpg.io/v1\nkind: Cluster\nmetadata: {name: database}\nspec:\n  inheritedMetadata: {labels: {app.kubernetes.io/part-of: app}}\n  imageName: ghcr.io/example/postgres@sha256:" + strings.Repeat("a", 64) + "\n  seccompProfile: {type: RuntimeDefault}\n  resources:\n    requests: {cpu: 10m, memory: 16Mi}\n    limits: {cpu: 100m, memory: 32Mi}\n"
	if err := templates.Safety(r, []byte(b), "test"); !errors.Is(err, templates.ErrCNPGProvider) {
		t.Fatalf("CNPG without its exact catalog provider: got %v", err)
	}
	r.Requires = []string{"databases/cloudnative-pg"}
	if err := templates.Safety(r, []byte(b), "test"); err != nil {
		t.Fatal(err)
	}
	unlabelled := strings.Replace(b, "  inheritedMetadata: {labels: {app.kubernetes.io/part-of: app}}\n", "", 1)
	if err := templates.Safety(r, []byte(unlabelled), "test"); !errors.Is(err, templates.ErrPartOf) {
		t.Fatalf("CNPG pods without the template label: got %v", err)
	}
}
func TestMiddlewareAdapterOnlyAllowsExplicitIPAllowList(t *testing.T) {
	r := &templates.Template{ID: "app", Requires: []string{"ingress/traefik"}}
	prefix := safeManifest + "---\napiVersion: traefik.io/v1alpha1\nkind: Middleware\nmetadata: {name: admin}\nspec:\n"
	if err := templates.Safety(r, []byte(prefix+"  ipAllowList: {sourceRange: [192.0.2.0/24]}\n"), "test"); err != nil {
		t.Fatal(err)
	}
	for spec, rule := range map[string]templates.Rule{
		"  ipAllowList: {sourceRange: [0.0.0.0/0]}\n":                                    templates.ErrIPAllowListCIDR,
		"  forwardAuth: {address: 'http://another-namespace'}\n":                         templates.ErrIPAllowListOnly,
		"  ipAllowList: {sourceRange: [192.0.2.0/24]}\n  stripPrefix: {prefixes: [/]}\n": templates.ErrIPAllowListOnly,
	} {
		if err := templates.Safety(r, []byte(prefix+spec), "test"); !errors.Is(err, rule) {
			t.Fatalf("%s: got %v, want %v", spec, err, rule)
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
		rule      templates.Rule
	}{
		{name: "complete", providers: []string{"ingress/traefik", "certificates/cert-manager"}},
		{name: "missing-certificates", providers: []string{"ingress/traefik"}, rule: templates.ErrClusterIssuerProvider},
		{name: "missing-ingress", providers: []string{"certificates/cert-manager"}, rule: templates.ErrTraefikProvider},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &templates.Template{ID: "app", Requires: tc.providers}
			if err := templates.Safety(r, []byte(manifest), "test"); !matches(err, tc.rule) {
				t.Fatalf("got %v, want %v", err, tc.rule)
			}
		})
	}
}

func TestOperatorDatabasesStayInTemplate(t *testing.T) {
	digest := "@sha256:" + strings.Repeat("a", 64)
	operand := `
  serviceAccountName: db
  resources:
    requests: {cpu: 10m, memory: 16Mi}
    limits: {cpu: 100m, memory: 32Mi}
  podSecurityContext: {runAsNonRoot: true, runAsUser: 999, seccompProfile: {type: RuntimeDefault}}
`
	dragonfly := safeManifest + `---
apiVersion: v1
kind: ServiceAccount
metadata: {name: db}
automountServiceAccountToken: false
---
apiVersion: dragonflydb.io/v1alpha1
kind: Dragonfly
metadata: {name: queue}
spec:
  image: docker.dragonflydb.io/dragonflydb/dragonfly` + digest + `
  args: [--cache_mode=false]
  podMetadata: {labels: {app.kubernetes.io/part-of: app}}
  authentication: {passwordFromSecret: {name: app, key: password}}
  containerSecurityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}` + operand
	mariadb := safeManifest + `---
apiVersion: v1
kind: ServiceAccount
metadata: {name: db}
automountServiceAccountToken: false
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata: {name: mariadb}
spec:
  image: docker.io/mariadb` + digest + `
  podMetadata: {labels: {app.kubernetes.io/part-of: app}}
  securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}` + operand + `---
apiVersion: k8s.mariadb.com/v1alpha1
kind: Grant
metadata: {name: app}
spec:
  mariaDbRef: {name: mariadb}
  username: app
  database: app
  privileges: [SELECT]
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata: {name: daily}
spec:
  mariaDbRef: {name: mariadb}
  podMetadata: {labels: {app.kubernetes.io/part-of: app}}
  securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}` + operand + `  storage:
    s3:
      bucket: backups
      endpoint: s3.example.test:443
      accessKeyIdSecretKeyRef: {name: s3, key: id}
      secretAccessKeySecretKeyRef: {name: s3, key: secret}
      tls: {enabled: true}
`
	for _, tc := range []struct {
		name, manifest, provider string
		rule                     templates.Rule
	}{
		{name: "dragonfly", manifest: dragonfly, provider: "databases/dragonfly-operator"},
		{name: "dragonfly-provider", manifest: dragonfly, provider: "databases/mariadb-operator", rule: templates.ErrDragonflyProvider},
		{name: "dragonfly-tag", manifest: strings.Replace(dragonfly, "dragonfly"+digest, "dragonfly:v1", 1), provider: "databases/dragonfly-operator", rule: templates.ErrDragonflyDigest},
		{name: "dragonfly-no-password", manifest: strings.Replace(dragonfly, "  authentication: {passwordFromSecret: {name: app, key: password}}\n", "", 1), provider: "databases/dragonfly-operator", rule: templates.ErrDragonflyPassword},
		{name: "dragonfly-memcached", manifest: strings.Replace(dragonfly, "--cache_mode=false", "--memcached_port=11211", 1), provider: "databases/dragonfly-operator", rule: templates.ErrDragonflyFlag},
		{name: "dragonfly-label", manifest: strings.Replace(dragonfly, "  podMetadata: {labels: {app.kubernetes.io/part-of: app}}\n", "", 1), provider: "databases/dragonfly-operator", rule: templates.ErrPartOf},
		{name: "dragonfly-sidecar", manifest: dragonfly + "  additionalContainers: [{name: x, image: x}]\n", provider: "databases/dragonfly-operator", rule: templates.ErrUnsupportedOverride},
		{name: "mariadb", manifest: mariadb, provider: "databases/mariadb-operator"},
		{name: "mariadb-provider", manifest: mariadb, provider: "databases/dragonfly-operator", rule: templates.ErrMariaDBProvider},
		{name: "mariadb-root", manifest: strings.Replace(mariadb, "runAsUser: 999", "runAsUser: 0", 1), provider: "databases/mariadb-operator", rule: templates.ErrOperandUIDZero},
		{name: "mariadb-config", manifest: strings.Replace(mariadb, "kind: MariaDB\nmetadata: {name: mariadb}\nspec:\n", "kind: MariaDB\nmetadata: {name: mariadb}\nspec:\n  myCnf: skip-grant-tables\n", 1), provider: "databases/mariadb-operator", rule: templates.ErrUnsupportedOverride},
		{name: "mariadb-other-namespace", manifest: strings.Replace(mariadb, "mariaDbRef: {name: mariadb}", "mariaDbRef: {name: mariadb, namespace: other}", 1), provider: "databases/mariadb-operator", rule: templates.ErrMariaDBRefNamespace},
		{name: "mariadb-undeclared", manifest: strings.Replace(mariadb, "mariaDbRef: {name: mariadb}", "mariaDbRef: {name: shared}", 1), provider: "databases/mariadb-operator", rule: templates.ErrMariaDBRefUndeclared},
		{name: "mariadb-global-grant", manifest: strings.Replace(mariadb, "database: app", "database: '*'", 1), provider: "databases/mariadb-operator", rule: templates.ErrGlobalGrant},
		{name: "mariadb-backup-plain", manifest: strings.Replace(mariadb, "tls: {enabled: true}", "tls: {enabled: false}", 1), provider: "databases/mariadb-operator", rule: templates.ErrMariaDBBackupS3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &templates.Template{ID: "app", Requires: []string{tc.provider}}
			r.Security.Risks = []map[string]string{{"risk": "Operator generated pods", "mitigation": "Pinned catalog review"}}
			if err := templates.Safety(r, []byte(tc.manifest), "test"); !matches(err, tc.rule) {
				t.Fatalf("got %v, want %v", err, tc.rule)
			}
		})
	}
}
