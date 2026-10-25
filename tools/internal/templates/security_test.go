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
      labels: {app: app}
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
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: default-deny}
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
  ingress: []
  egress: []
`

// matches reports whether err breaks rule, or is nil when rule is empty.
func matches(err error, rule templates.Rule) bool {
	if rule == "" {
		return err == nil
	}
	return errors.Is(err, rule)
}

func TestSafetyRejectsPrivilegeAndNetworkEscapes(t *testing.T) {
	r := &templates.Template{}
	r.Services = []templates.Service{{Name: "app", SecurityProfile: "restricted"}}
	if err := templates.Safety(r, []byte(safeManifest), "test"); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		manifest string
		rule     templates.Rule
	}{
		"token":                           {strings.Replace(safeManifest, "automountServiceAccountToken: false", "automountServiceAccountToken: true", 1), templates.ErrPodTokenAutomount},
		"privileged":                      {strings.Replace(safeManifest, "allowPrivilegeEscalation: false", "allowPrivilegeEscalation: false\n            privileged: true", 1), templates.ErrPrivilegedContainer},
		"container-overrides-pod-nonroot": {strings.Replace(safeManifest, "allowPrivilegeEscalation: false", "allowPrivilegeEscalation: false\n            runAsNonRoot: false", 1), templates.ErrRunAsNonRoot},
		"hostnamespace":                   {strings.Replace(safeManifest, "automountServiceAccountToken: false", "automountServiceAccountToken: false\n      hostPID: true", 1), templates.ErrHostNamespace},
		"hostpath":                        {strings.Replace(safeManifest, "automountServiceAccountToken: false", "automountServiceAccountToken: false\n      volumes: [{name: host, hostPath: {path: /}}]", 1), templates.ErrHostPathVolume},
		"lifecycle-host":                  {strings.Replace(safeManifest, "          readinessProbe:", "          lifecycle: {preStop: {httpGet: {host: 127.0.0.1, path: /, port: 10250}}}\n          readinessProbe:", 1), templates.ErrHostLifecycleHook},
		"tag":                             {strings.Replace(safeManifest, "example.test/app@sha256:"+strings.Repeat("a", 64), "example.test/app:latest", 1), templates.ErrImageDigest},
		"namespace":                       {strings.Replace(safeManifest, "name: app\nspec:", "name: app\n  namespace: kube-system\nspec:", 1), templates.ErrNamespaceEscape},
		"network":                         {strings.Replace(safeManifest, "policyTypes: [Ingress, Egress]", "policyTypes: [Ingress]", 1), templates.ErrDefaultDeny},
		"rbac":                            {safeManifest + "---\napiVersion: rbac.authorization.k8s.io/v1\nkind: RoleBinding\nmetadata: {name: escape}\n", templates.ErrMaintenanceBindingRole},
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
	if err := templates.Safety(r, []byte(b), "test"); !errors.Is(err, templates.ErrRunAsNonRoot) {
		t.Fatalf("baseline service downgraded unrelated restricted app: %v", err)
	}
}
func TestRenderRequiresLicenseAcknowledgement(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifests.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata: {name: app}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := &templates.Template{}
	if err := yaml.Unmarshal([]byte(`{"services":[{"name":"app"}],"inputs":[{"name":"licenseOptIn","description":"license acknowledgement","required":true,"validation":{"pattern":"^I-ACCEPT-LICENSE$"}}]}`), r); err != nil {
		t.Fatal(err)
	}
	r.Dir = dir
	opts := templates.RenderOptions{Namespace: "test", Inputs: map[string]string{}}
	if _, err := templates.Render(r, opts); !errors.Is(err, templates.ErrMissingInput) {
		t.Fatalf("missing acknowledgement: got %v", err)
	}
	opts.Inputs["licenseOptIn"] = "no"
	if _, err := templates.Render(r, opts); !errors.Is(err, templates.ErrInputPattern) {
		t.Fatalf("invalid acknowledgement: got %v", err)
	}
	opts.Inputs["licenseOptIn"] = "I-ACCEPT-LICENSE"
	if _, err := templates.Render(r, opts); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedAllowsIsolatedPingWithoutAllowingHostSysctls(t *testing.T) {
	r := &templates.Template{}
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
