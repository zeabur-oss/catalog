package catalog

import (
	"regexp"
	"strings"
	"testing"
)

func logsManifests(t *testing.T) string {
	t.Helper()
	cat, err := Load("../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cat.Components {
		if c.ID == "victoria-logs" {
			if r, ok := c.Release("logs"); ok {
				return r.Manifests
			}
		}
	}
	t.Fatal("victoria-logs has no logs release")
	return ""
}

// The collector leaves out Kubernetes' and its own namespace, and keeps Zeabur's components
// and the projects.
func TestLogCollectionLeavesOutOnlyInfrastructure(t *testing.T) {
	m := regexp.MustCompile(`kubernetes\.pod_namespace:in\(([^)]*)\)`).FindStringSubmatch(logsManifests(t))
	if m == nil {
		t.Fatal("victoria-logs has no namespace excludeFilter")
	}
	excluded := map[string]bool{}
	for _, ns := range strings.Split(m[1], ",") {
		excluded[strings.TrimSpace(ns)] = true
	}
	for _, ns := range []string{"kube-system", "observability"} {
		if !excluded[ns] {
			t.Errorf("%s is still collected", ns)
		}
	}
	for _, ns := range []string{"zeabur", "zeabur-identity", "traefik", "cert-manager"} {
		if excluded[ns] {
			t.Errorf("%s is left out, but its logs are kept", ns)
		}
	}
}

// kube-apiserver's audit log is one distinct stream, read from either distribution's directory.
func TestLogCollectionReadsTheAuditLog(t *testing.T) {
	manifests := logsManifests(t)
	for _, want := range []string{
		"fileCollector.glob: /var/lib/rancher/*/server/logs/audit.log",
		`"app":"kube-apiserver-audit"`,
		"fileCollector.timeField: stageTimestamp",
	} {
		if !strings.Contains(manifests, want) {
			t.Errorf("victoria-logs does not set %s", want)
		}
	}
}
