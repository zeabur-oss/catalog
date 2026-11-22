package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOnlyAnAdditionKeepsV1(t *testing.T) {
	const before = `{
		"$defs": {"entry": {"type": "object", "required": ["id"], "properties": {
			"id": {"type": "string"}, "size": {"type": "integer"}}}},
		"type": "object", "required": ["entries"],
		"properties": {"entries": {"type": "array", "items": {"$ref": "#/$defs/entry"}}}
	}`
	for _, c := range []struct {
		name, from, to, want string
	}{
		{"an added field and value", `"size": {"type": "integer"}`, `"size": {"type": "integer"}, "note": {"type": "string", "enum": ["a"]}`, ""},
		{"a removed field", `, "size": {"type": "integer"}`, ``, "properties size is removed"},
		{"a changed type", `"size": {"type": "integer"}`, `"size": {"type": "string"}`, `type "integer" is now "string"`},
		{"a changed $ref", `"#/$defs/entry"`, `"#/$defs/other"`, `$ref "#/$defs/entry" is now "#/$defs/other"`},
		{"a newly required field", `"required": ["id"]`, `"required": ["id", "size"]`, "size is newly required"},
		{"a removed definition", `"entry": {`, `"other": {`, "$defs entry is removed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var b, a map[string]any
			if err := json.Unmarshal([]byte(before), &b); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(strings.Replace(before, c.from, c.to, 1)), &a); err != nil {
				t.Fatal(err)
			}
			got := strings.Join(breaks("s.json", b, a), "\n")
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("breaks = %q, want %q", got, c.want)
			}
		})
	}
}
