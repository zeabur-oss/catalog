package catalog

import (
	"errors"
	"fmt"
	"io"
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

	licenses, err := ReadLicenses(root)
	if err != nil {
		return err
	}

	for _, c := range cat.Components {
		for _, p := range licenses.problems(c) {
			fail(c.Path, "%s", p)
		}
		for _, r := range c.Releases {
			if len(r.URLDigests) != len(r.URLs) {
				fail(c.Path, "%s: urlDigests needs one digest for each of the %d urls", r.Name, len(r.URLs))
			}
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
			if in.Fill != "" && in.Path == "" {
				fail(c.Path, "input %s: fill needs a path", in.Name)
			}
		}
		for _, p := range c.provisionProblems() {
			fail(c.Path, "%s", p)
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
	presets := map[string]string{}
	for _, c := range cat.Components {
		for _, p := range c.Presets {
			key := c.Capability + " " + p
			if other, ok := presets[key]; ok {
				fail(c.Path, "preset %s: %s already installs a %s component", p, other, c.Capability)
			}
			presets[key] = c.ID
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
			fail("components", "%s: %v", p, err)
		}
	}
	return errors.Join(errs...)
}

// provisionProblems checks what provides and networkPolicies name against the
// component's own inputs and namespaces.
func (c *Component) provisionProblems() []string {
	var out []string
	owns := func(ns string) bool {
		return slices.ContainsFunc(c.Releases, func(r Release) bool { return r.Namespace == ns })
	}
	if r := c.Provides.registry(); r != nil {
		if !owns(r.CA.Namespace) {
			out = append(out, fmt.Sprintf("provides.registry.ca: %s is none of its namespaces", r.CA.Namespace))
		}
		if r.ClusterIPInput != "" && !slices.ContainsFunc(c.Inputs, func(in Input) bool {
			return in.Name == r.ClusterIPInput && in.Path != ""
		}) {
			out = append(out, fmt.Sprintf("provides.registry.clusterIPInput: no input %s with a path", r.ClusterIPInput))
		}
	}
	dec := yaml.NewDecoder(strings.NewReader(c.NetworkPolicies))
	for {
		var o struct {
			Kind     string `yaml:"kind"`
			Metadata struct{ Name, Namespace string }
		}
		if err := dec.Decode(&o); err != nil {
			if !errors.Is(err, io.EOF) {
				out = append(out, "networkPolicies: "+err.Error())
			}
			break
		}
		if o.Kind != "NetworkPolicy" || !owns(o.Metadata.Namespace) {
			out = append(out, fmt.Sprintf("networkPolicies: %s %s must be a NetworkPolicy in one of its namespaces", o.Kind, o.Metadata.Name))
		}
	}
	return out
}

func (p *Provides) registry() *Registry {
	if p == nil {
		return nil
	}
	return p.Registry
}

// Licenses is config/licenses.yaml.
type Licenses struct {
	OSIApproved []string `yaml:"osiApproved"`
	// OptInOnly are allowed although not OSI-approved, in components that no
	// profile recommends.
	OptInOnly []string `yaml:"optInOnly"`
}

func ReadLicenses(root string) (Licenses, error) {
	var l Licenses
	return l, ReadYAML(filepath.Join(root, "config", "licenses.yaml"), &l)
}

// NotOSIApproved returns the identifiers in an SPDX expression that are not
// OSI-approved.
func (l Licenses) NotOSIApproved(expr string) []string {
	var out []string
	for _, id := range licenseIDs(expr) {
		if !slices.Contains(l.OSIApproved, id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// licenseProblem is a licence that a component cannot use as it is set up. An
// empty Profile means the licence is not OSI-approved; otherwise the licence is
// opt-in only and Profile recommends the component.
type licenseProblem struct {
	License, Profile string
}

func (p licenseProblem) Error() string {
	if p.Profile == "" {
		return fmt.Sprintf("license %s is not OSI-approved", p.License)
	}
	return fmt.Sprintf("license %s is opt-in only, so %s cannot be recommended", p.License, p.Profile)
}

func (l Licenses) problems(c *Component) []licenseProblem {
	var out []licenseProblem
	for _, id := range l.NotOSIApproved(c.Metadata.License) {
		if !slices.Contains(l.OptInOnly, id) {
			out = append(out, licenseProblem{License: id})
			continue
		}
		for _, p := range Profiles {
			if c.Defaults[p] == "recommended" {
				out = append(out, licenseProblem{License: id, Profile: p})
			}
		}
	}
	return out
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
