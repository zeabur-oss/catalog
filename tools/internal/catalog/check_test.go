package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLicenseProblems(t *testing.T) {
	l := Licenses{OSIApproved: []string{"Apache-2.0", "MIT"}, OptInOnly: []string{"SSPL-1.0"}}
	optional := map[string]string{"single-node": "optional", "multi-node": "optional"}
	tests := []struct {
		license  string
		defaults map[string]string
		want     []licenseProblem
	}{
		{"Apache-2.0 OR MIT", map[string]string{"single-node": "recommended"}, nil},
		{"Apache-2.0 AND SSPL-1.0", optional, nil},
		{"Apache-2.0 AND SSPL-1.0", map[string]string{"single-node": "optional", "multi-node": "recommended"},
			[]licenseProblem{{License: "SSPL-1.0", Profile: "multi-node"}}},
		{"BUSL-1.1", optional, []licenseProblem{{License: "BUSL-1.1"}}},
	}
	for _, tt := range tests {
		got := l.problems(&Component{Metadata: Metadata{License: tt.license}, Defaults: tt.defaults})
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s %v: got %v, want %v", tt.license, tt.defaults, got, tt.want)
		}
	}
	if got := l.NotOSIApproved("(MIT AND SSPL-1.0) OR SSPL-1.0"); !reflect.DeepEqual(got, []string{"SSPL-1.0"}) {
		t.Errorf("NotOSIApproved: got %q", got)
	}
}

func TestProvisionProblems(t *testing.T) {
	component := func(edit func(*Component)) *Component {
		c := &Component{
			Releases:  []Release{{Name: "zot", Namespace: "zot"}},
			Inputs:    []Input{{Name: "clusterIp", Path: "service.clusterIP", Generate: &Generate{Type: GenerateClusterIP}}},
			Provides:  &Provides{Registry: &Registry{Host: "zot.zot.svc:5000", CA: SecretRef{Name: "zot-tls", Key: "ca.crt", Namespace: "zot"}}},
			Detection: Detection{CRDs: []string{"reports.example.com", "jobs.example.com"}},
		}
		edit(c)
		return c
	}
	tests := []struct {
		name string
		c    *Component
		bad  bool
	}{
		{"its own CRDs, inputs and namespaces", component(func(c *Component) {
			c.InternalCRDs = []string{"reports.example.com"}
			c.NetworkPolicies = "kind: NetworkPolicy\nmetadata: {name: a, namespace: zot}\n"
		}), false},
		{"an internal CRD it does not detect", component(func(c *Component) { c.InternalCRDs = []string{"other.example.com"} }), true},
		{"a generated ClusterIP without a path", component(func(c *Component) { c.Inputs[0].Path = ""; c.Inputs[0].Secret = &SecretRef{} }), true},
		{"a CA in another namespace", component(func(c *Component) { c.Provides.Registry.CA.Namespace = "default" }), true},
		{"a policy in another namespace", component(func(c *Component) {
			c.NetworkPolicies = "kind: NetworkPolicy\nmetadata: {name: a, namespace: default}\n"
		}), true},
		{"another kind than NetworkPolicy", component(func(c *Component) {
			c.NetworkPolicies = "kind: ConfigMap\nmetadata: {name: a, namespace: zot}\n"
		}), true},
	}
	for _, tt := range tests {
		if got := tt.c.provisionProblems(); (len(got) > 0) != tt.bad {
			t.Errorf("%s: got %q", tt.name, got)
		}
	}
}

func TestHealthChecksNameTheirObjectsOneWay(t *testing.T) {
	const root = "../../.."
	const path = "components/traefik.yaml"
	cat, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(root, cat); err != nil {
		t.Fatalf("the catalog does not validate: %v", err)
	}
	valid := func() map[string]any {
		return map[string]any{"name": "controller",
			"resource":  map[string]any{"kind": "Deployment", "name": "traefik", "namespace": "traefik"},
			"condition": "Available", "timeout": "60s", "advice": "Check the pods."}
	}
	resource := func(h map[string]any) map[string]any { return h["resource"].(map[string]any) }
	for _, c := range []struct {
		name string
		ok   bool
		edit func(map[string]any)
	}{
		{"by name", true, func(map[string]any) {}},
		{"by selector", true, func(h map[string]any) {
			delete(resource(h), "name")
			resource(h)["selector"] = map[string]any{"app.kubernetes.io/name": "traefik"}
		}},
		{"all", true, func(h map[string]any) { delete(resource(h), "name"); resource(h)["all"] = true }},
		{"cluster-scoped kind", true, func(h map[string]any) {
			h["resource"] = map[string]any{"kind": "GatewayClass", "name": "traefik"}
		}},
		{"unknown kind", false, func(h map[string]any) { resource(h)["kind"] = "ConfigMap" }},
		{"no condition", false, func(h map[string]any) { delete(h, "condition") }},
		{"no advice", false, func(h map[string]any) { delete(h, "advice") }},
		{"no timeout", false, func(h map[string]any) { delete(h, "timeout") }},
		{"no object", false, func(h map[string]any) { delete(resource(h), "name") }},
		{"name and all", false, func(h map[string]any) { resource(h)["all"] = true }},
		{"all false", false, func(h map[string]any) { delete(resource(h), "name"); resource(h)["all"] = false }},
		{"empty selector", false, func(h map[string]any) {
			delete(resource(h), "name")
			resource(h)["selector"] = map[string]any{}
		}},
		{"namespaced kind without namespace", false, func(h map[string]any) { delete(resource(h), "namespace") }},
		{"cluster-scoped kind with namespace", false, func(h map[string]any) {
			h["resource"] = map[string]any{"kind": "GatewayClass", "name": "traefik", "namespace": "traefik"}
		}},
		{"kubectl arguments", false, func(h map[string]any) { h["wait"] = []any{"--for=condition=Ready", "pod"} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := valid()
			c.edit(h)
			raw := cat.Raw[path].(map[string]any)
			old := raw["health"]
			raw["health"] = []any{h}
			defer func() { raw["health"] = old }()
			err := Check(root, cat)
			if c.ok && err != nil {
				t.Errorf("Check rejected a valid health check: %v", err)
			}
			if !c.ok && err == nil {
				t.Error("Check accepted the health check")
			}
		})
	}
}

func TestHealthCheckRunsAsOneKubectlWait(t *testing.T) {
	check := func(r HealthResource) Health {
		return Health{Resource: r, Condition: "Ready", Timeout: "60s"}
	}
	for _, c := range []struct {
		name  string
		check Health
		want  string
	}{
		{"by name", check(HealthResource{Kind: "Deployment", Name: "web", Namespace: "web"}),
			"wait --for=condition=Ready deployments.apps/web -n web --timeout=60s"},
		{"by selector", check(HealthResource{Kind: "Pod", Selector: map[string]string{"b": "2", "a": "1"}, Namespace: "web"}),
			"wait --for=condition=Ready pods -l a=1,b=2 -n web --timeout=60s"},
		{"all", check(HealthResource{Kind: "Node.longhorn.io", All: true, Namespace: "longhorn-system"}),
			"wait --for=condition=Ready nodes.longhorn.io --all -n longhorn-system --timeout=60s"},
		{"cluster-scoped", check(HealthResource{Kind: "APIService", Name: "v1beta1.metrics.k8s.io"}),
			"wait --for=condition=Ready apiservices.apiregistration.k8s.io/v1beta1.metrics.k8s.io --timeout=60s"},
	} {
		if got := strings.Join(c.check.WaitArgs(), " "); got != c.want {
			t.Errorf("%s: kubectl %s, want kubectl %s", c.name, got, c.want)
		}
	}
}

func TestKubeVersionMustCoverTheTestedVersions(t *testing.T) {
	for _, c := range []struct {
		name, kubeVersion, version string
		ok                         bool
	}{
		{"chart floor below the tested version", ">=1.25.0-0", "1.36.0", true},
		{"distribution release", ">=1.34.0-0", "v1.36.5+k3s1", true},
		{"tested version left out", ">=1.37.0-0", "1.36.0", false},
		{"not a range", "1.3x", "1.36.0", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := checkKubeVersion(c.kubeVersion, c.version); (err == nil) != c.ok {
				t.Errorf("checkKubeVersion(%q, %q) = %v, want ok %v", c.kubeVersion, c.version, err, c.ok)
			}
		})
	}
}

func TestCapabilityMustNotBeAnotherComponentsID(t *testing.T) {
	const root = "../../.."
	cat, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	traefik, _ := cat.Get("traefik")
	traefik.Provides.Capability = "cert-manager"
	if err := Check(root, cat); err == nil {
		t.Error("Check accepted a capability that is the ID of another component")
	}
}

func TestLoadRefusesCapabilityFolders(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "components", "ingress")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "traefik.yaml"), []byte("kind: Component\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Error("Load accepted components/ingress/traefik.yaml")
	}
}
