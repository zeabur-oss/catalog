package templates_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

const root = "../../.."

func TestTemplateCases(t *testing.T) {
	schemaPath, err := filepath.Abs(filepath.Join(root, "schema", "v1", "template-case.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := jsonschema.NewCompiler().Compile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, "conformance", "template", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no cases: %v", err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			var c map[string]any
			if err := catalog.ReadYAML(f, &c); err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(c); err != nil {
				t.Fatal(err)
			}
			if _, ok := c["manifests"]; ok {
				runRender(t, c)
			}
		})
	}
}

// runRender renders the template of a case with Render and compares the
// objects, or the code and input of the refusal, with want.
func runRender(t *testing.T, c map[string]any) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifests.yaml"), []byte(c["manifests"].(string)), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := yaml.Marshal(c["template"])
	if err != nil {
		t.Fatal(err)
	}
	r := &templates.Template{}
	if err := yaml.Unmarshal(file, r); err != nil {
		t.Fatal(err)
	}
	r.ID, r.Dir = c["id"].(string), dir
	var opts templates.RenderOptions
	if err := remarshal(c["options"], &opts); err != nil {
		t.Fatal(err)
	}
	var got any
	out, err := templates.Render(r, opts)
	if check := (*templates.CheckError)(nil); errors.As(err, &check) {
		e := map[string]any{"code": check.Code}
		if check.Input != "" {
			e["input"] = check.Input
		}
		got = map[string]any{"error": e}
	} else if err != nil {
		t.Fatal(err)
	} else {
		objects, err := templates.DecodeDocuments(out)
		if err != nil {
			t.Fatal(err)
		}
		got = map[string]any{"objects": objects}
	}
	var gotJSON, wantJSON any
	if err := remarshal(got, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if err := remarshal(c["want"], &wantJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Errorf("got %v, want %v", gotJSON, wantJSON)
	}
}

func remarshal(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
