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
// reader of v1 ignores fields it does not know, so the published schemas are checked as if no
// object were closed: a new field passes, a missing or changed one does not.
func checkCompat(published, api string) error {
	open, err := openSchemas(filepath.Join(published, "schema"))
	if err != nil {
		return err
	}
	defer os.RemoveAll(open)
	if err := checkAPI(api, open); err != nil {
		return fmt.Errorf("a change inside v1 must keep passing the published schemas; a breaking change goes to v2:\n%w", err)
	}
	return checkAPI(api, filepath.Join(api, "schema"))
}

// openSchemas copies the schemas in dir to a temporary directory without
// "additionalProperties": false.
func openSchemas(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		return "", err
	}
	out, err := os.MkdirTemp("", "catalog-compat-")
	if err != nil {
		return "", err
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return out, err
		}
		var schema any
		if err := json.Unmarshal(data, &schema); err != nil {
			return out, fmt.Errorf("%s: %w", f, err)
		}
		data, err = json.Marshal(withoutClosedObjects(schema))
		if err != nil {
			return out, err
		}
		if err := os.WriteFile(filepath.Join(out, filepath.Base(f)), data, 0o644); err != nil {
			return out, err
		}
	}
	return out, nil
}

func withoutClosedObjects(v any) any {
	switch v := v.(type) {
	case map[string]any:
		if closed, ok := v["additionalProperties"].(bool); ok && !closed {
			delete(v, "additionalProperties")
		}
		for k, e := range v {
			v[k] = withoutClosedObjects(e)
		}
	case []any:
		for i, e := range v {
			v[i] = withoutClosedObjects(e)
		}
	}
	return v
}
