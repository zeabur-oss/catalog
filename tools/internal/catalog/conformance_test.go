package catalog

import (
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestGeneratedInputsStateLengthAndEncoding(t *testing.T) {
	schemaPath, err := filepath.Abs(filepath.Join("../../..", "schema", "v1", "component.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := jsonschema.NewCompiler().Compile(schemaPath + "#/$defs/Generate")
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
