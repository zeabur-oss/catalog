package templates_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

const safeManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  template:
    metadata:
      labels: {app: app, templates.zeabur.com/template: app}
    spec:
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: app
          image: example.test/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
          securityContext:
            allowPrivilegeEscalation: false
            capabilities: {drop: [ALL]}
          resources:
            requests: {cpu: 10m, memory: 16Mi}
            limits: {cpu: 100m, memory: 32Mi}
          readinessProbe: {tcpSocket: {port: 8080}}
          livenessProbe: {tcpSocket: {port: 8080}}
`

// allow returns a NetworkPolicy that allows ingress to the template's pods from peer.
func allow(peer string) string {
	return "---\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata: {name: allow}\nspec:\n  podSelector: {matchLabels: {templates.zeabur.com/template: app}}\n  policyTypes: [Ingress]\n  ingress: [{from: [" + peer + "]}]\n"
}

// matches reports whether err breaks rule, or is nil when rule is empty.
func matches(err error, rule templates.Rule) bool {
	if rule == "" {
		return err == nil
	}
	return errors.Is(err, rule)
}

func TestSafetyRejectsPrivilegeAndNetworkEscapes(t *testing.T) {
	r := &templates.Template{ID: "app"}
	r.Services = []templates.Service{{Name: "app", SecurityProfile: "restricted"}}
	if err := templates.Safety(r, []byte(safeManifest+allow("{namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: traefik}}, podSelector: {matchLabels: {app: traefik}}}")), "test"); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		manifest string
		rule     templates.Rule
	}{
		"token":                           {strings.Replace(safeManifest, "automountServiceAccountToken: false", "automountServiceAccountToken: true", 1), templates.ErrPodTokenAutomount},
		"privileged":                      {strings.Replace(safeManifest, "allowPrivilegeEscalation: false", "allowPrivilegeEscalation: false\n            privileged: true", 1), templates.ErrPodSecurity},
		"container-overrides-pod-nonroot": {strings.Replace(safeManifest, "allowPrivilegeEscalation: false", "allowPrivilegeEscalation: false\n            runAsNonRoot: false", 1), templates.ErrPodSecurity},
		"hostnamespace":                   {strings.Replace(safeManifest, "automountServiceAccountToken: false", "automountServiceAccountToken: false\n      hostPID: true", 1), templates.ErrPodSecurity},
		"hostpath":                        {strings.Replace(safeManifest, "automountServiceAccountToken: false", "automountServiceAccountToken: false\n      volumes: [{name: host, hostPath: {path: /}}]", 1), templates.ErrPodSecurity},
		"lifecycle-host":                  {strings.Replace(safeManifest, "          readinessProbe:", "          lifecycle: {preStop: {httpGet: {host: 127.0.0.1, path: /, port: 10250}}}\n          readinessProbe:", 1), templates.ErrPodSecurity},
		"tag":                             {strings.Replace(safeManifest, "example.test/app@sha256:"+strings.Repeat("a", 64), "example.test/app:latest", 1), templates.ErrImageDigest},
		"namespace":                       {strings.Replace(safeManifest, "name: app\nspec:", "name: app\n  namespace: kube-system\nspec:", 1), templates.ErrNamespaceEscape},
		"pod-label":                       {strings.Replace(safeManifest, "labels: {app: app, templates.zeabur.com/template: app}", "labels: {app: app}", 1), templates.ErrTemplateLabel},
		"peer-without-label":              {safeManifest + allow("{podSelector: {matchLabels: {app: other}}}"), templates.ErrForeignPods},
		"peer-whole-namespace":            {safeManifest + allow("{namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: test}}}"), templates.ErrForeignPods},
		"storage-class":                   {safeManifest + "---\napiVersion: v1\nkind: PersistentVolumeClaim\nmetadata: {name: data}\nspec: {storageClassName: local-path}\n", templates.ErrStorageClass},
		"rbac":                            {safeManifest + "---\napiVersion: rbac.authorization.k8s.io/v1\nkind: RoleBinding\nmetadata: {name: escape}\n", templates.ErrUnsupportedKind},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := templates.Safety(r, []byte(tc.manifest), "test"); !errors.Is(err, tc.rule) {
				t.Fatalf("got %v, want %v", err, tc.rule)
			}
		})
	}
	// One baseline service must not downgrade an unrelated restricted workload.
	r.Services = append(r.Services, templates.Service{Name: "legacy", SecurityProfile: "baseline", BaselineJustification: "legacy UID"})
	b := strings.Replace(safeManifest, "runAsNonRoot: true", "runAsNonRoot: false", 1)
	if err := templates.Safety(r, []byte(b), "test"); !errors.Is(err, templates.ErrPodSecurity) {
		t.Fatalf("baseline service downgraded unrelated restricted app: %v", err)
	}
}
func TestRenderChecksRequiredInputPattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifests.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata: {name: app}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &templates.Template{ID: "app"}
	if err := yaml.Unmarshal([]byte(`{"services":[{"name":"app"}],"inputs":[{"name":"region","description":"region name","required":true,"validation":{"pattern":"^eu-[a-z]+$"}}]}`), r); err != nil {
		t.Fatal(err)
	}
	r.Dir = dir
	opts := templates.RenderOptions{Namespace: "test", Inputs: map[string]string{}}
	if _, err := templates.Render(r, opts); !errors.Is(err, templates.ErrMissingInput) {
		t.Fatalf("missing input: got %v", err)
	}
	opts.Inputs["region"] = "no"
	if _, err := templates.Render(r, opts); !errors.Is(err, templates.ErrInputPattern) {
		t.Fatalf("invalid input: got %v", err)
	}
	opts.Inputs["region"] = "eu-west"
	if _, err := templates.Render(r, opts); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedAllowsIsolatedPingWithoutAllowingHostSysctls(t *testing.T) {
	r := &templates.Template{ID: "app"}
	r.Services = []templates.Service{{Name: "app", SecurityProfile: "restricted"}}
	for _, tc := range []struct {
		name, value string
		rule        templates.Rule
	}{
		{name: "net.ipv4.ping_group_range", value: "1000 1000"},
		{name: "kernel.core_pattern", value: "/tmp/core", rule: templates.ErrUnsafeSysctl},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := strings.Replace(safeManifest,
				"securityContext:\n        runAsNonRoot: true",
				"securityContext:\n        sysctls: [{name: "+tc.name+", value: \""+tc.value+"\"}]\n        runAsNonRoot: true", 1)
			if err := templates.Safety(r, []byte(b), "test"); !matches(err, tc.rule) {
				t.Fatalf("got %v, want %v", err, tc.rule)
			}
		})
	}
}

func TestUnauthenticatedServiceNeedsIngressPolicy(t *testing.T) {
	r := &templates.Template{ID: "app", Services: []templates.Service{{Name: "app"}, {Name: "cache"}}}
	if err := templates.Safety(r, []byte(safeManifest), "test"); err != nil {
		t.Fatalf("no policy for a service that has its own sign-in: %v", err)
	}
	r.Security.Unauthenticated = []string{"app"}
	if err := templates.Safety(r, []byte(safeManifest), "test"); !errors.Is(err, templates.ErrIngressPolicy) {
		t.Fatalf("unauthenticated service without policy: got %v", err)
	}
	named := strings.Replace(allow("{podSelector: {matchLabels: {templates.zeabur.com/template: app}}}"), "template: app}}\n  policyTypes", "template: app, app: app}}\n  policyTypes", 1)
	if err := templates.Safety(r, []byte(safeManifest+named), "test"); err != nil {
		t.Fatalf("unauthenticated service with policy: %v", err)
	}
	other := &templates.Template{ID: "app", Services: r.Services, Security: templates.Security{Unauthenticated: []string{"cache"}}}
	if err := templates.Safety(other, []byte(safeManifest+named), "test"); !errors.Is(err, templates.ErrIngressPolicy) {
		t.Fatalf("policy that names another service: got %v", err)
	}
	other.Security.Unauthenticated = []string{"missing"}
	if err := templates.Safety(other, []byte(safeManifest), "test"); !errors.Is(err, templates.ErrUnknownService) {
		t.Fatalf("unknown service: got %v", err)
	}
}
