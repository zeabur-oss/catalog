package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// schema writes schema/v1/ from the Go types.
func schema(fs *flag.FlagSet, _ *repo) error {
	fs.Parse(os.Args[2:])
	files, err := schemaFiles()
	if err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join("schema", "v1", name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// schemaFiles returns the schema of each file format, by file name: the component and template
// files people write, and the files of the API.
func schemaFiles() (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, f := range []struct {
		tag, name, title string
		v                any
	}{
		{"yaml", "component.schema.json", "Component", &catalog.Component{}},
		{"yaml", "template.schema.json", "Template", &templates.Template{}},
		{"json", "catalog.schema.json", "catalog.json", &CatalogFile{}},
		{"json", "component-file.schema.json", "components/<id>.json", &ComponentFile{}},
		{"json", "template-file.schema.json", "templates/<id>.json", &TemplateFile{}},
	} {
		r, err := catalog.NewReflector(f.tag)
		if err != nil {
			return nil, err
		}
		data, err := catalog.Reflect(r, f.v, f.name, f.title)
		if err != nil {
			return nil, err
		}
		out[f.name] = data
	}
	return out, nil
}
