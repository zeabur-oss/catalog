// Package catalog loads, checks, renders and tests the component catalog.
package catalog

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Profiles every component describes.
var Profiles = []string{"single-node", "multi-node"}

type Component struct {
	ID              string            `yaml:"-" json:"id"`
	Capability      string            `yaml:"-" json:"capability"`
	DisplayName     string            `yaml:"displayName" json:"displayName"`
	Homepage        string            `yaml:"homepage" json:"homepage"`
	License         string            `yaml:"license" json:"license"`
	Maturity        string            `yaml:"maturity" json:"maturity"`
	Stability       string            `yaml:"stability" json:"stability,omitempty"`
	Rationale       string            `yaml:"rationale" json:"rationale"`
	Alternatives    map[string]string `yaml:"alternatives" json:"alternatives,omitempty"`
	Defaults        map[string]string `yaml:"defaults" json:"defaults"`
	Distributions   []string          `yaml:"distributions" json:"distributions,omitempty"`
	PersistsData    bool              `yaml:"persistsData" json:"persistsData,omitempty"`
	DisruptsTraffic bool              `yaml:"disruptsTraffic" json:"disruptsTraffic,omitempty"`
	DependsOn       []string          `yaml:"dependsOn" json:"dependsOn,omitempty"`
	Inputs          []Input           `yaml:"inputs" json:"inputs,omitempty"`
	Releases        []Release         `yaml:"releases" json:"releases"`
	Contributes     []Contribution    `yaml:"contributes" json:"contributes,omitempty"`
	Detection       map[string]any    `yaml:"detection" json:"detection"`
	Security        map[string]any    `yaml:"security" json:"security"`
	Smoke           []Smoke           `yaml:"smoke" json:"smoke,omitempty"`
	Leftovers       []string          `yaml:"leftovers" json:"leftovers,omitempty"`
	UpgradeNotes    []map[string]any  `yaml:"upgradeNotes" json:"upgradeNotes,omitempty"`

	Path string `yaml:"-" json:"-"` // relative to the repository root
}

type Release struct {
	Name          string            `yaml:"name" json:"name"`
	Namespace     string            `yaml:"namespace" json:"namespace"`
	ReleaseName   string            `yaml:"releaseName" json:"releaseName,omitempty"`
	SkipIfPresent []string          `yaml:"skipIfPresent" json:"skipIfPresent,omitempty"`
	Chart         string            `yaml:"chart" json:"chart,omitempty"`
	Repo          string            `yaml:"repo" json:"repo,omitempty"`
	URLs          []string          `yaml:"urls" json:"urls,omitempty"`
	Version       string            `yaml:"version" json:"version,omitempty"`
	Manifests     string            `yaml:"manifests" json:"manifests,omitempty"`
	Values        string            `yaml:"values" json:"values,omitempty"`
	Profiles      map[string]string `yaml:"profiles" json:"profiles,omitempty"`
}

type Input struct {
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description" json:"description"`
	Release     string     `yaml:"release" json:"release,omitempty"`
	Path        string     `yaml:"path" json:"path,omitempty"`
	Secret      *SecretRef `yaml:"secret" json:"secret,omitempty"`
	Generate    string     `yaml:"generate" json:"generate,omitempty"`
	Required    bool       `yaml:"required" json:"required,omitempty"`
	Default     any        `yaml:"default" json:"default,omitempty"`
	TestValue   any        `yaml:"testValue" json:"testValue,omitempty"`
}

type SecretRef struct {
	Name      string `yaml:"name" json:"name"`
	Key       string `yaml:"key" json:"key"`
	Namespace string `yaml:"namespace" json:"namespace"`
}

type Contribution struct {
	Target  string `yaml:"target" json:"target"`
	Release string `yaml:"release" json:"release"`
	Values  string `yaml:"values" json:"values"`
}

type Smoke struct {
	Name     string   `yaml:"name" json:"name"`
	Manifest string   `yaml:"manifest" json:"manifest,omitempty"`
	Wait     []string `yaml:"wait" json:"wait"`
	Timeout  string   `yaml:"timeout" json:"timeout,omitempty"`
}

// HelmName is the Helm release name.
func (r Release) HelmName() string { return cmp.Or(r.ReleaseName, r.Name) }

// Kind is helm, manifest or inline.
func (r Release) Kind() string {
	switch {
	case r.Chart != "":
		return "helm"
	case len(r.URLs) > 0:
		return "manifest"
	}
	return "inline"
}

// Release returns the named release.
func (c *Component) Release(name string) (Release, bool) {
	i := slices.IndexFunc(c.Releases, func(r Release) bool { return r.Name == name })
	if i < 0 {
		return Release{}, false
	}
	return c.Releases[i], true
}

// Catalog is every component in the repository, sorted by ID.
type Catalog struct {
	Components []*Component
	// Raw holds each file decoded generically, for schema validation.
	Raw map[string]any
}

// Load reads capabilities/<capability>/<upstream>.yaml under root.
func Load(root string) (*Catalog, error) {
	files, err := filepath.Glob(filepath.Join(root, "capabilities", "*", "*.yaml"))
	if err != nil {
		return nil, err
	}
	cat := &Catalog{Raw: map[string]any{}}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(root, f)
		var raw any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		cat.Raw[rel] = raw
		c := &Component{Path: rel}
		if err := yaml.Unmarshal(data, c); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		c.Capability = filepath.Base(filepath.Dir(f))
		c.ID = c.Capability + "/" + strings.TrimSuffix(filepath.Base(f), ".yaml")
		cat.Components = append(cat.Components, c)
	}
	slices.SortFunc(cat.Components, func(a, b *Component) int { return cmp.Compare(a.ID, b.ID) })
	return cat, nil
}

// Get returns the component with the given ID.
func (cat *Catalog) Get(id string) (*Component, bool) {
	i := slices.IndexFunc(cat.Components, func(c *Component) bool { return c.ID == id })
	if i < 0 {
		return nil, false
	}
	return cat.Components[i], true
}

// provider resolves a reference for a profile: <capability>/<upstream> as is,
// <capability> to its recommended upstream, else an optional one.
func (cat *Catalog) provider(ref, profile string) (*Component, error) {
	if strings.Contains(ref, "/") {
		if c, ok := cat.Get(ref); ok {
			return c, nil
		}
		return nil, fmt.Errorf("unknown component %q", ref)
	}
	var optional *Component
	for _, c := range cat.Components {
		if c.Capability != ref {
			continue
		}
		switch c.Defaults[profile] {
		case "recommended":
			return c, nil
		case "optional":
			optional = cmp.Or(optional, c)
		}
	}
	if optional == nil {
		return nil, fmt.Errorf("nothing provides %q on %s", ref, profile)
	}
	return optional, nil
}

// InstallOrder returns ids and their dependencies, dependencies first.
func (cat *Catalog) InstallOrder(ids []string, profile string) ([]*Component, error) {
	var order []*Component
	state := map[*Component]int{} // 1 visiting, 2 done
	var visit func(c *Component) error
	visit = func(c *Component) error {
		switch state[c] {
		case 1:
			return fmt.Errorf("dependency cycle through %s", c.ID)
		case 2:
			return nil
		}
		state[c] = 1
		for _, ref := range c.DependsOn {
			dep, err := cat.provider(ref, profile)
			if err != nil {
				return fmt.Errorf("%s: %w", c.ID, err)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[c] = 2
		order = append(order, c)
		return nil
	}
	for _, id := range ids {
		c, err := cat.provider(id, profile)
		if err != nil {
			return nil, err
		}
		if err := visit(c); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// values returns a chart release's values for a profile: shared values, the
// profile's overrides, then inputs set at their paths. Comments are dropped.
func (c *Component) values(rel Release, profile string, inputs map[string]any) (map[string]any, error) {
	vals := map[string]any{}
	for _, doc := range []string{rel.Values, rel.Profiles[profile]} {
		var m map[string]any
		if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", c.ID, rel.Name, err)
		}
		merge(vals, m)
	}
	for _, in := range c.Inputs {
		v, ok := inputs[in.Name]
		if !ok || in.Path == "" || c.inputRelease(in) != rel.Name {
			continue
		}
		if err := setPath(vals, strings.Split(in.Path, "."), v); err != nil {
			return nil, fmt.Errorf("%s input %s: %w", c.ID, in.Name, err)
		}
	}
	return vals, nil
}

func (c *Component) inputRelease(in Input) string {
	if in.Release != "" {
		return in.Release
	}
	for _, r := range c.Releases {
		if r.Kind() == "helm" {
			return r.Name
		}
	}
	return ""
}

// testInputs returns the values CI installs with: testValue, else default.
func (c *Component) testInputs() map[string]any {
	out := map[string]any{}
	for _, in := range c.Inputs {
		switch {
		case in.TestValue != nil:
			out[in.Name] = in.TestValue
		case in.Default != nil:
			out[in.Name] = in.Default
		}
	}
	return out
}

// merge merges src into dst like Helm layers values files: maps merge,
// everything else replaces.
func merge(dst, src map[string]any) {
	for k, v := range src {
		if sm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				merge(dm, sm)
				continue
			}
			v = maps.Clone(sm)
		}
		dst[k] = v
	}
}

// setPath sets v at a dotted path; numeric segments index lists.
func setPath(node any, path []string, v any) error {
	key, rest := path[0], path[1:]
	switch n := node.(type) {
	case map[string]any:
		if len(rest) == 0 {
			n[key] = v
			return nil
		}
		if n[key] == nil {
			if _, err := strconv.Atoi(rest[0]); err == nil {
				n[key] = []any{}
			} else {
				n[key] = map[string]any{}
			}
		}
		if l, ok := n[key].([]any); ok {
			i, err := strconv.Atoi(rest[0])
			if err != nil {
				return fmt.Errorf("%s is a list", key)
			}
			for len(l) <= i {
				l = append(l, map[string]any{})
			}
			n[key] = l
			if len(rest) == 1 {
				l[i] = v
				return nil
			}
			return setPath(l[i], rest[1:], v)
		}
		return setPath(n[key], rest, v)
	}
	return fmt.Errorf("cannot set %s inside a %T", key, node)
}
