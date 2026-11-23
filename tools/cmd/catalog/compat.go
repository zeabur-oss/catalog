package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// compat fails unless every file of the API in <api> passes the schemas of the published API in
// <published>, such as the latest release's v1/, and its own. A reader of v1 checks the data, not
// the text of the schemas, so the data is what must keep passing.
func compat(fs *flag.FlagSet, _ *repo) error {
	fs.Parse(os.Args[2:])
	if fs.NArg() != 2 {
		return errors.New("usage: catalog compat <dir of a published v1> <dir of the new v1>")
	}
	return checkCompat(fs.Arg(0), fs.Arg(1))
}

// checkCompat checks the API in api against the schemas of published, then against its own. A
// reader of v1 ignores a field it does not know and handles a value it does not know, so the
// published schemas are checked without closed objects and enums: a new field or value passes, a
// missing or retyped field does not.
func checkCompat(published, api string) error {
	open, err := os.MkdirTemp("", "catalog-compat-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(open)
	if err := openSchemas(filepath.Join(published, "schema"), open); err != nil {
		return err
	}
	if err := checkAPI(api, open); err != nil {
		return fmt.Errorf("a change inside v1 must keep passing the published schemas; a breaking change goes to v2:\n%w", err)
	}
	return checkAPI(api, filepath.Join(api, "schema"))
}

// openSchemas copies the schemas in dir to out without "additionalProperties": false and enums.
func openSchemas(dir, out string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		return err
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var schema any
		if err := json.Unmarshal(data, &schema); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		data, err = json.Marshal(opened(schema))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, filepath.Base(f)), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// opened drops "additionalProperties": false, "enum", and a "oneOf" whose every branch is a
// "const", which is how the generated schemas write an enum. An "if" or a "not" stays as written:
// opened, it would match other data, so "then" would apply to every value, or "not" to none.
func opened(v any) any {
	schema, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if closed, ok := schema["additionalProperties"].(bool); ok && !closed {
		delete(schema, "additionalProperties")
	}
	delete(schema, "enum")
	if branches, ok := schema["oneOf"].([]any); ok && allConst(branches) {
		delete(schema, "oneOf")
	}
	for k, e := range schema {
		switch k {
		case "properties", "patternProperties", "$defs":
			if named, ok := e.(map[string]any); ok {
				for name, s := range named {
					named[name] = opened(s)
				}
			}
		case "allOf", "anyOf", "oneOf", "prefixItems":
			if list, ok := e.([]any); ok {
				for i, s := range list {
					list[i] = opened(s)
				}
			}
		case "items", "additionalProperties", "then", "else", "contains", "propertyNames":
			schema[k] = opened(e)
		}
	}
	return schema
}

func allConst(branches []any) bool {
	for _, b := range branches {
		m, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := m["const"]; !ok {
			return false
		}
	}
	return len(branches) > 0
}
