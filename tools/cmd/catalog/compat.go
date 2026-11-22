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

func readSchema(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s map[string]any
	return s, json.Unmarshal(data, &s)
}

// breaks lists what a reader of before cannot read in after: a field, definition or branch
// removed, a type or $ref changed, or a field newly required.
func breaks(path string, before, after map[string]any) []string {
	var out []string
	for _, key := range []string{"type", "$ref"} {
		b, ok := before[key]
		if !ok {
			continue
		}
		if a, ok := after[key]; !ok {
			out = append(out, fmt.Sprintf("%s: %s %s is removed", path, key, mustJSON(b)))
		} else if mustJSON(a) != mustJSON(b) {
			out = append(out, fmt.Sprintf("%s: %s %s is now %s", path, key, mustJSON(b), mustJSON(a)))
		}
	}
	required := func(s map[string]any) []any { r, _ := s["required"].([]any); return r }
	for _, r := range required(after) {
		if !slices.Contains(required(before), r) {
			out = append(out, fmt.Sprintf("%s: %v is newly required", path, r))
		}
	}
	for _, key := range []string{"properties", "$defs"} {
		b, _ := before[key].(map[string]any)
		a, _ := after[key].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(b)) {
			if _, ok := a[name]; !ok {
				out = append(out, fmt.Sprintf("%s: %s %s is removed", path, key, name))
				continue
			}
			// A schema can also be true; it has nothing to compare.
			bs, _ := b[name].(map[string]any)
			as, _ := a[name].(map[string]any)
			out = append(out, breaks(path+"/"+key+"/"+name, bs, as)...)
		}
	}
	// A schema added here, such as a new then, is compared with an empty one: it must not require.
	for _, key := range []string{"items", "additionalProperties", "then", "else"} {
		b, _ := before[key].(map[string]any)
		if a, ok := after[key].(map[string]any); ok || b != nil {
			out = append(out, breaks(path+"/"+key, b, a)...)
		}
	}
	// The generator writes the branches in a fixed order, so they compare by index. A new anyOf
	// or oneOf branch accepts more, and a new allOf branch must not require.
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		b, _ := before[key].([]any)
		a, _ := after[key].([]any)
		for i := range max(len(b), len(a)) {
			switch {
			case i >= len(a):
				out = append(out, fmt.Sprintf("%s: %s/%d is removed", path, key, i))
			case i < len(b) || key == "allOf":
				bs, _ := at(b, i).(map[string]any)
				as, _ := a[i].(map[string]any)
				out = append(out, breaks(fmt.Sprintf("%s/%s/%d", path, key, i), bs, as)...)
			}
		}
	}
	return out
}

func at(list []any, i int) any {
	if i < len(list) {
		return list[i]
	}
	return nil
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
