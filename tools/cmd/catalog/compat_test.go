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
	// A published schema that needs a field the new data lacks is a break for its readers.
	file := filepath.Join(published, "schema", "catalog.schema.json")
	data, err := os.ReadFile(file)
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
