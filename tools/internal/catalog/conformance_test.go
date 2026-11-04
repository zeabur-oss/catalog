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

func TestGeneratedInputsStateLengthAndEncoding(t *testing.T) {
	schemaPath, err := filepath.Abs(filepath.Join("../../..", "schema", "v1", "component.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := jsonschema.NewCompiler().Compile(schemaPath + "#/$defs/generate")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		generate map[string]any
		ok       bool
	}{
		{map[string]any{"type": "random", "bytes": 64, "encoding": "hex"}, true},
		{map[string]any{"type": "random"}, false},
		{map[string]any{"type": "random", "bytes": 64}, false},
		{map[string]any{"type": "clusterIP"}, true},
		{map[string]any{"type": "clusterIP", "bytes": 64}, false},
	} {
		if err := schema.Validate(c.generate); (err == nil) != c.ok {
			t.Errorf("%v: %v, want ok %v", c.generate, err, c.ok)
		}
	}
}
