package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		if got := strings.Join(c.check.Wait().Args("", c.check.Timeout), " "); got != c.want {
			t.Errorf("%s: kubectl %s, want kubectl %s", c.name, got, c.want)
		}
	}
}

func TestLoadRefusesAComponentFileOutsideItsDirectory(t *testing.T) {
	for _, c := range []struct {
		files []string
		ok    bool
	}{
		{[]string{"traefik/component.yaml", "traefik/README.md", "traefik/README.zh-TW.md"}, true},
		{[]string{"traefik.yaml"}, false},
		{[]string{"traefik/component.yaml", "traefik.README.md"}, false},
		{[]string{"ingress/traefik.yaml"}, false},
		{[]string{"traefik/component.yaml", "traefik/values.yaml"}, false},
		{[]string{"traefik/component.yaml", "traefik/charts/values.yaml"}, false},
	} {
		root := t.TempDir()
		for _, f := range c.files {
			path := filepath.Join(root, "components", f)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("kind: Component\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Load(root); (err == nil) != c.ok {
			t.Errorf("Load of %v: error %v, want ok %v", c.files, err, c.ok)
		}
	}
}
