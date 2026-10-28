package catalog

import (
	"reflect"
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
			Inputs:    []Input{{Name: "cluster-ip", Path: "service.clusterIP", Generate: "clusterIP"}},
			Provides:  &Provides{Registry: &Registry{Host: "zot.zot.svc:5000", CA: SecretRef{Name: "zot-tls", Key: "ca.crt", Namespace: "zot"}}},
			Detection: map[string]any{"crds": []any{"reports.example.com", "jobs.example.com"}},
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
