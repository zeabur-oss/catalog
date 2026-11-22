package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOnlyAnAdditionKeepsV1(t *testing.T) {
	const before = `{
		"$defs": {"entry": {"type": "object", "required": ["id"], "description": "An entry.",
			"if": {"required": ["id"]}, "then": {},
			"properties": {
				"id": {"type": "string"},
				"size": {"type": "integer"},
				"any": true,
				"kind": {"type": "string", "enum": ["a", "b"]},
				"cpu": {"anyOf": [{"type": "string"}, {"type": "integer"}]},
				"gen": {"oneOf": [{"required": ["a"]}, {"required": ["b"]}]}}}},
		"type": "object", "required": ["entries"],
		"properties": {"entries": {"type": "array", "items": {"$ref": "#/$defs/entry"}}}
	}`
	for _, c := range []struct {
		name, from, to, want string
	}{
		{"an added field", `"size": {"type": "integer"},`, `"size": {"type": "integer"}, "note": {"type": "string"},`, ""},
		{"an added value", `["a", "b"]`, `["a", "b", "c"]`, ""},
		{"an added anyOf branch", `{"type": "string"}, {"type": "integer"}`, `{"type": "string"}, {"type": "integer"}, {"type": "null"}`, ""},
		{"a changed description", `"An entry."`, `"One entry."`, ""},
		{"a removed field", `"size": {"type": "integer"},`, ``, "properties/size is removed"},
		{"a removed value", `["a", "b"]`, `["a"]`, "enum: b is removed"},
		{"a changed type", `"size": {"type": "integer"}`, `"size": {"type": "string"}`, `size/type: "integer" is now "string"`},
		{"a type on a field that took anything", `"any": true`, `"any": {"type": "string"}`, `any: true is now {"type":"string"}`},
		{"a field that takes nothing", `"any": true`, `"any": false`, `any: true is now false`},
		{"a changed $ref", `"#/$defs/entry"`, `"#/$defs/other"`, `$ref: "#/$defs/entry" is now "#/$defs/other"`},
		{"a newly required field", `"required": ["id"], "description"`, `"required": ["id", "size"], "description"`, "required: size is new"},
		{"a changed type in a branch", `{"type": "string"}, {"type": "integer"}`, `{"type": "boolean"}, {"type": "integer"}`, `anyOf/0/type: "string" is now "boolean"`},
		{"a removed branch", `{"type": "string"}, {"type": "integer"}`, `{"type": "string"}`, "anyOf: 2 branches are now 1"},
		{"an added oneOf branch", `{"required": ["b"]}]`, `{"required": ["b"]}, {}]`, `oneOf: [{"required":["a"]},{"required":["b"]}] is now [{"required":["a"]},{"required":["b"]},{}]`},
		{"an added allOf", `"type": "object", "required": ["id"]`, `"type": "object", "allOf": [{"required": ["size"]}], "required": ["id"]`, "allOf: 0 branches are now 1"},
		{"a then that requires", `"then": {}`, `"then": {"required": ["size"]}`, "then/required: size is new"},
		{"a looser oneOf branch", `{"required": ["a"]}, {"required": ["b"]}`, `{}, {"required": ["b"]}`, `oneOf: [{"required":["a"]}`},
		{"a changed if", `"if": {"required": ["id"]}`, `"if": {}`, `if: {"required":["id"]} is now {}`},
		{"a removed definition", `"entry": {`, `"other": {`, "$defs/entry is removed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			after := strings.Replace(before, c.from, c.to, 1)
			if after == before {
				t.Fatalf("%q is not in the schema", c.from)
			}
			var b, a any
			if err := json.Unmarshal([]byte(before), &b); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(after), &a); err != nil {
				t.Fatal(err)
			}
			got := strings.Join(breaks("s.json", b, a), "\n")
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("breaks = %q, want %q", got, c.want)
			}
		})
	}
}
