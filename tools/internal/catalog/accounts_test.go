package catalog

import (
	"strings"
	"testing"
)

func TestNamespacesGetADefaultAccountBeforeTheirReleases(t *testing.T) {
	c := &Component{Releases: []Release{
		{Name: "app", Namespace: "app", Chart: "oci://example/app"},
		{Name: "extra", Namespace: "app", Manifests: "x"},
		{Name: "dns", Namespace: "kube-system", Manifests: "y"},
		{Name: "tools", Namespace: "tools", Manifests: "z"},
	}}
	c.addDefaultAccounts()

	var accounts []Release
	for i, r := range c.Releases {
		if strings.Contains(r.Manifests, "name: default\n") {
			accounts = append(accounts, r)
			if i >= 2 {
				t.Errorf("%s comes after a release", r.Name)
			}
			if !strings.Contains(r.Manifests, "automountServiceAccountToken: false") {
				t.Errorf("%s mounts a token", r.Name)
			}
		}
	}
	got := map[string]bool{}
	for _, r := range accounts {
		got[r.Namespace] = true
	}
	if len(accounts) != 2 || !got["app"] || !got["tools"] {
		t.Errorf("accounts for %v, want app and tools once each, not kube-system", got)
	}
}
