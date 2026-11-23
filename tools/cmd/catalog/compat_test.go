package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatChecksTheDataAgainstThePublishedSchemas(t *testing.T) {
	writeTestAPI(t)
	published := t.TempDir()
	if err := os.CopyFS(filepath.Join(published, "schema"), os.DirFS("out/v1/schema")); err != nil {
		t.Fatal(err)
	}
	if err := checkCompat(published, "out/v1"); err != nil {
		t.Fatalf("the same schemas: %v", err)
	}
	// A reader ignores a field it does not know and handles a value it does not know, so a new
	// field or profile passes, though the published schemas as written refuse them.
	open := t.TempDir()
	if err := openSchemas(filepath.Join(published, "schema"), open); err != nil {
		t.Fatal(err)
	}
	catalogJSON := filepath.Join("out", "v1", "catalog.json")
	data, err := os.ReadFile(catalogJSON)
	if err != nil {
		t.Fatal(err)
	}
	for name, changed := range map[string]string{
		"a new field":   strings.Replace(string(data), "{", `{"added": true, `, 1),
		"a new profile": strings.Replace(string(data), `"profiles": [`, `"profiles": ["edge", `, 1),
	} {
		if changed == string(data) {
			t.Fatalf("%s: nothing changed", name)
		}
		if err := os.WriteFile(catalogJSON, []byte(changed), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := checkAPI("out/v1", filepath.Join(published, "schema")); err == nil {
			t.Fatalf("%s: the published schema as written must refuse it", name)
		}
		if err := checkAPI("out/v1", open); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := os.WriteFile(catalogJSON, data, 0o644); err != nil {
		t.Fatal(err)
	}
	// A published schema that needs a field the new data lacks is a break for its readers.
	file := filepath.Join(published, "schema", "catalog.schema.json")
	data, err = os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	narrowed := strings.Replace(string(data), `"required": [`, `"required": ["gone", `, 1)
	if err := os.WriteFile(file, []byte(narrowed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkCompat(published, "out/v1"); err == nil || !strings.Contains(err.Error(), "catalog.json") {
		t.Fatalf("got %v, want catalog.json to fail the published schema", err)
	}
}
