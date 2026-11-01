package catalog

import (
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestSelectCasesMatchTheirSchema(t *testing.T) {
	const root = "../../.."
	schemaPath, err := filepath.Abs(filepath.Join(root, "schema", "v1", "select-case.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := jsonschema.NewCompiler().Compile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, "conformance", "select", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no cases: %v", err)
	}
	for _, f := range files {
		var c any
		if err := ReadYAML(f, &c); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if err := schema.Validate(c); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}
