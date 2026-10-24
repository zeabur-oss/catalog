package catalog

import (
	"reflect"
	"testing"
)

func TestValues(t *testing.T) {
	c := &Component{
		ID: "x/y",
		Releases: []Release{{
			Name:  "r",
			Chart: "oci://example/r",
			Values: `
a: {b: 1, c: [1, 2]}
list: [{name: first}]`,
			Profiles: map[string]string{"single-node": `a: {c: [3]}`},
		}},
		Inputs: []Input{{Name: "bucket", Path: "list.0.bucket"}},
	}
	got, err := c.values(c.Releases[0], "single-node", map[string]any{"bucket": "b1"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"a":    map[string]any{"b": 1, "c": []any{3}},
		"list": []any{map[string]any{"name": "first", "bucket": "b1"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestInstallOrder(t *testing.T) {
	cat := &Catalog{Components: []*Component{
		{ID: "a/x", Capability: "a", Defaults: map[string]string{"single-node": "recommended"}, Requires: []string{"b"}},
		{ID: "b/y", Capability: "b", Defaults: map[string]string{"single-node": "optional"}},
	}}
	order, err := cat.InstallOrder([]string{"a/x"}, "single-node")
	if err != nil || len(order) != 2 || order[0].ID != "b/y" {
		t.Fatalf("got %v, %v", order, err)
	}
	cat.Components[1].Requires = []string{"a/x"}
	if _, err := cat.InstallOrder([]string{"a/x"}, "single-node"); err == nil {
		t.Error("cycle not detected")
	}
}

func TestUnexplained(t *testing.T) {
	got := unexplained(`
# why a
a: 1
b: 2
c:
  # covers the block
  d: 1
  e: 2
f: 3 # why f
`)
	if !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("got %v", got)
	}
}

func TestLicenseIDs(t *testing.T) {
	got := licenseIDs("(MIT OR Apache-2.0) AND GPL-2.0+")
	if !reflect.DeepEqual(got, []string{"MIT", "Apache-2.0", "GPL-2.0"}) {
		t.Errorf("got %v", got)
	}
}

func TestReplace(t *testing.T) {
	got, err := replace([]byte("server: https://prod/dir\n---\nserver: https://prod/dir"),
		map[string]string{"https://prod/dir": "https://staging/dir"})
	if want := "server: https://staging/dir\n---\nserver: https://staging/dir"; err != nil || string(got) != want {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}
	if _, err := replace([]byte("server: x"), map[string]string{"absent": "y"}); err == nil {
		t.Error("a replacement that matches nothing must fail")
	}
	if got, err := replace([]byte("a"), nil); err != nil || string(got) != "a" {
		t.Errorf("no replacements: got %q, %v", got, err)
	}
}
