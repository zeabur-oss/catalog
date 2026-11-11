package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// A template is linted against the CRDs of what it installs only, so a custom
// resource of a component it does not require fails kubeconform.
func TestTemplateLintSeesOnlyTheCRDsOfWhatItInstalls(t *testing.T) {
	dir := t.TempDir()
	for _, g := range []string{"a", "b"} {
		crd := "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nspec:\n  group: " + g + ".example.com\n" +
			"  names: {kind: Thing}\n  versions:\n    - name: v1\n      schema:\n        openAPIV3Schema: {type: object, properties: {spec: {type: object}}}\n"
		f := filepath.Join(dir, g, "crds.yaml")
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(crd), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	schemas, err := orderSchemas(dir, []*catalog.Component{{ID: "a"}})
	defer os.RemoveAll(schemas)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(schemas, "a.example.com", "thing_v1.json")); err != nil {
		t.Errorf("no schema of the required component: %v", err)
	}
	if _, err := os.Stat(filepath.Join(schemas, "b.example.com")); !os.IsNotExist(err) {
		t.Errorf("a schema of a component the template does not install: %v", err)
	}
}
