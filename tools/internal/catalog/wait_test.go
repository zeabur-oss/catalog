package catalog

import (
	"slices"
	"testing"
)

func TestWaitNamesItsObjectsToKubectl(t *testing.T) {
	for _, c := range []struct {
		name     string
		wait     Wait
		args     []string
		describe []string
		ns       string
	}{
		{"one object", Wait{Resource: "certificate/smoke", Namespace: "app", Condition: "condition=Ready"},
			[]string{"wait", "--for=condition=Ready", "certificate/smoke", "-n", "app", "--timeout=1m"},
			[]string{"-n", "app", "certificate/smoke"}, "app"},
		{"selector", Wait{Resource: "pod", Namespace: "app", Selector: map[string]string{"b": "2", "a": "1"}, Condition: "condition=Ready"},
			[]string{"wait", "--for=condition=Ready", "pod", "-l", "a=1,b=2", "-n", "app", "--timeout=1m"},
			[]string{"-n", "app", "pod", "-l", "a=1,b=2"}, "app"},
		{"all", Wait{Resource: "vmsingle", Namespace: "app", All: true, Condition: "jsonpath={.status.phase}=ok"},
			[]string{"wait", "--for=jsonpath={.status.phase}=ok", "vmsingle", "--all", "-n", "app", "--timeout=1m"},
			[]string{"-n", "app", "vmsingle"}, "app"},
		{"cluster-scoped", Wait{Resource: "crd/x.example.com", Condition: "condition=Established"},
			[]string{"wait", "--for=condition=Established", "crd/x.example.com", "--timeout=1m"},
			[]string{"crd/x.example.com"}, ""},
	} {
		if got := c.wait.Args("", "1m"); !slices.Equal(got, c.args) {
			t.Errorf("%s: wait %v, want %v", c.name, got, c.args)
		}
		if object, ns := c.wait.Describe(""); !slices.Equal(object, c.describe) || ns != c.ns {
			t.Errorf("%s: describe %v in %q, want %v in %q", c.name, object, ns, c.describe, c.ns)
		}
	}
}

func TestWaitUsesTheDefaultNamespaceOfItsCheck(t *testing.T) {
	w := Wait{Resource: "job/smoke", Condition: "condition=Complete"}
	if got := w.Args("catalog-smoke", "1m"); !slices.Contains(got, "catalog-smoke") {
		t.Errorf("wait %v does not use the namespace of the check", got)
	}
}
