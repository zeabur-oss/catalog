package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

// Check validates every component without a cluster: schema, licenses,
// references and value comments. Each problem is one joined error.
func Check(root string, cat *Catalog) error {
	var errs []error
	fail := func(path, format string, a ...any) {
		errs = append(errs, fmt.Errorf("%s: %s", path, fmt.Sprintf(format, a...)))
	}

	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	schemaPath, err := filepath.Abs(filepath.Join(root, "schema", "v1", "component.schema.json"))
	if err != nil {
		return err
	}
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return err
	}
	for _, c := range cat.Components {
		if err := schema.Validate(cat.Raw[c.Path]); err != nil {
			fail(c.Path, "%v", err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...) // later checks assume a valid shape
	}

	var licenses struct {
		OSIApproved []string `yaml:"osiApproved"`
	}
	if err := ReadYAML(filepath.Join(root, "config", "licenses.yaml"), &licenses); err != nil {
		return err
	}

	for _, c := range cat.Components {
		for _, id := range licenseIDs(c.License) {
			if !slices.Contains(licenses.OSIApproved, id) {
				fail(c.Path, "license %s is not OSI-approved", id)
			}
		}
		for _, r := range c.Releases {
			check := func(where, values string) {
				for _, p := range unexplained(values) {
					fail(c.Path, "%s: %s has no comment saying why", where, p)
				}
			}
			check(r.Name, r.Values)
			for p, v := range r.Profiles {
				check(r.Name+" "+p, v)
			}
		}
		for _, in := range c.Inputs {
			if in.Path != "" && c.inputRelease(in) == "" {
				fail(c.Path, "input %s: no chart release for its path", in.Name)
			}
			if in.Required && in.Default == nil && in.TestValue == nil && in.Generate == "" {
				fail(c.Path, "input %s: required, so set default, testValue or generate for CI to install it", in.Name)
			}
		}
		for _, ct := range c.Contributes {
			t, ok := cat.Get(ct.Target)
			if !ok {
				fail(c.Path, "contributes: unknown target %s", ct.Target)
			} else if _, ok := t.Release(ct.Release); !ok {
				fail(c.Path, "contributes: %s has no release %s", ct.Target, ct.Release)
			}
			for _, p := range unexplained(ct.Values) {
				fail(c.Path, "contributes to %s: %s has no comment saying why", ct.Target, p)
			}
		}
	}
	for _, p := range Profiles {
		var ids []string
		for _, c := range cat.Components {
			if c.Defaults[p] != "unsupported" {
				ids = append(ids, c.ID)
			}
		}
		if _, err := cat.InstallOrder(ids, p); err != nil {
			fail("capabilities", "%s: %v", p, err)
		}
	}
	return errors.Join(errs...)
}

// licenseIDs splits an SPDX expression; every identifier must be approved,
// including both sides of an OR.
func licenseIDs(expr string) []string {
	var out []string
	for _, tok := range strings.Fields(strings.NewReplacer("(", " ", ")", " ").Replace(expr)) {
		if tok != "AND" && tok != "OR" && tok != "WITH" {
			out = append(out, strings.TrimSuffix(tok, "+"))
		}
	}
	return out
}

// unexplained returns the paths of values without a comment on themselves or
// an ancestor. Inside a nested block, a comment on the first entry covers the
// block; at the document root each key needs its own.
func unexplained(values string) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(values), &doc); err != nil {
		return []string{"(invalid YAML: " + err.Error() + ")"}
	}
	if len(doc.Content) == 0 {
		return nil
	}
	var out []string
	var walk func(n *yaml.Node, path string, explained bool)
	walk = func(n *yaml.Node, path string, explained bool) {
		explained = explained || commented(n) || (path != "" && firstCommented(n))
		switch n.Kind {
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i]
				walk(n.Content[i+1], strings.TrimPrefix(path+"."+k.Value, "."), explained || commented(k))
			}
		case yaml.SequenceNode:
			for i, item := range n.Content {
				walk(item, fmt.Sprintf("%s[%d]", path, i), explained)
			}
		default:
			if !explained {
				out = append(out, path)
			}
		}
	}
	walk(doc.Content[0], "", commented(&doc))
	return out
}

func commented(n *yaml.Node) bool {
	return strings.TrimSpace(n.HeadComment+n.LineComment) != ""
}

// firstCommented reports whether a block's first entry has a head comment.
func firstCommented(n *yaml.Node) bool {
	if (n.Kind != yaml.MappingNode && n.Kind != yaml.SequenceNode) || len(n.Content) == 0 {
		return false
	}
	first := n.Content[0]
	return first.HeadComment != "" ||
		first.Kind == yaml.MappingNode && len(first.Content) > 0 && first.Content[0].HeadComment != ""
}

func ReadYAML(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}
