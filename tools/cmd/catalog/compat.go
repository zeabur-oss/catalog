package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// compat fails unless schema/v1/ only adds to the schemas of the API in <dir>/schema/, such as a
// published release's v1/: it keeps every field and type, and requires no new field.
func compat(fs *flag.FlagSet, _ *repo) error {
	fs.Parse(os.Args[2:])
	if fs.NArg() != 1 {
		return errors.New("usage: catalog compat <dir of a published v1>")
	}
	var problems []string
	for _, name := range apiSchemas {
		before, err := readSchema(filepath.Join(fs.Arg(0), "schema", name))
		if errors.Is(err, os.ErrNotExist) {
			continue // a schema the release did not have yet
		}
		if err != nil {
			return err
		}
		after, err := readSchema(filepath.Join("schema", "v1", name))
		if err != nil {
			return err
		}
		problems = append(problems, breaks(name, before, after)...)
	}
	if len(problems) > 0 {
		return fmt.Errorf("a change inside v1 must only add; a breaking change goes to v2:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func readSchema(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s any
	return s, json.Unmarshal(data, &s)
}

// breaks lists what changed from before to after other than an addition. An addition is a new
// property where before forbade other fields, a new definition, a new branch of an existing anyOf,
// a new value of an existing enum, a field no longer required, or a new description. Inside an
// if, a not or a oneOf nothing may change. Anything else, such as a removed field, a changed type
// or a new required field, can break a reader.
func breaks(path string, before, after any) []string {
	b, bok := before.(map[string]any)
	a, aok := after.(map[string]any)
	if !bok || !aok {
		if mustJSON(before) == mustJSON(after) {
			return nil
		}
		return []string{fmt.Sprintf("%s: %s is now %s", path, mustJSON(before), mustJSON(after))}
	}
	var out []string
	keys := slices.Sorted(maps.Keys(b))
	for k := range a {
		if _, ok := b[k]; !ok {
			keys = append(keys, k)
		}
	}
	for _, key := range keys {
		at := path + "/" + key
		switch key {
		case "description":
		case "if", "not", "oneOf":
			// Loosening these can tighten the schema (a then applies to more, a oneOf matches
			// twice), so any change to them counts.
			if mustJSON(b[key]) != mustJSON(a[key]) {
				out = append(out, fmt.Sprintf("%s: %s is now %s", at, mustJSON(b[key]), mustJSON(a[key])))
			}
		case "properties", "$defs":
			bm, _ := b[key].(map[string]any)
			am, _ := a[key].(map[string]any)
			for _, name := range slices.Sorted(maps.Keys(bm)) {
				if _, ok := am[name]; !ok {
					out = append(out, fmt.Sprintf("%s/%s is removed", at, name))
				} else {
					out = append(out, breaks(at+"/"+name, bm[name], am[name])...)
				}
			}
			// A new property constrains a field that before let through, unless before forbade
			// every other field.
			if key == "properties" && b["additionalProperties"] != false {
				for _, name := range slices.Sorted(maps.Keys(am)) {
					if _, ok := bm[name]; !ok {
						out = append(out, fmt.Sprintf("%s/%s is new", at, name))
					}
				}
			}
		case "required":
			for _, v := range asList(a[key]) {
				if !contains(asList(b[key]), v) {
					out = append(out, fmt.Sprintf("%s: %v is new", at, v))
				}
			}
		case "enum", "anyOf", "allOf":
			if _, ok := b[key]; !ok {
				out = append(out, fmt.Sprintf("%s: %s is new", at, mustJSON(a[key])))
				continue
			}
			if key == "enum" {
				for _, v := range asList(b[key]) {
					if !contains(asList(a[key]), v) {
						out = append(out, fmt.Sprintf("%s: %v is removed", at, v))
					}
				}
			} else {
				out = append(out, branches(at, key, asList(b[key]), asList(a[key]))...)
			}
		default:
			out = append(out, breaks(at, b[key], a[key])...)
		}
	}
	return out
}

// branches compares anyOf or allOf branches by index: the generator writes them in a fixed order.
// An anyOf may gain branches; an allOf may not.
func branches(path, key string, before, after []any) []string {
	if len(after) < len(before) || key == "allOf" && len(after) != len(before) {
		return []string{fmt.Sprintf("%s: %d branches are now %d", path, len(before), len(after))}
	}
	var out []string
	for i := range before {
		out = append(out, breaks(fmt.Sprintf("%s/%d", path, i), before[i], after[i])...)
	}
	return out
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func contains(l []any, v any) bool {
	return slices.ContainsFunc(l, func(x any) bool { return mustJSON(x) == mustJSON(v) })
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
