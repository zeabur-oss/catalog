package catalog

import (
	"regexp"
	"strings"
	"testing"
)

// VictoriaLogs keeps project logs only, so every namespace the catalog installs into is left out.
func TestLogCollectionLeavesOutEveryComponentNamespace(t *testing.T) {
	cat, err := Load("../../..")
	if err != nil {
		t.Fatal(err)
	}
	var manifests string
	for _, c := range cat.Components {
		if c.ID == "logs/victoria-logs" {
			if r, ok := c.Release("logs"); ok {
				manifests = r.Manifests
			}
		}
	}
	m := regexp.MustCompile(`kubernetes\.pod_namespace:in\(([^)]*)\)`).FindStringSubmatch(manifests)
	if m == nil {
		t.Fatal("logs/victoria-logs has no namespace excludeFilter")
	}
	excluded := map[string]bool{}
	for _, ns := range strings.Split(m[1], ",") {
		excluded[strings.TrimSpace(ns)] = true
	}
	for _, c := range cat.Components {
		for _, r := range c.Releases {
			if r.Namespace != "" && !excluded[r.Namespace] {
				t.Errorf("%s installs into %s, which VictoriaLogs still collects", c.ID, r.Namespace)
			}
		}
	}
}
