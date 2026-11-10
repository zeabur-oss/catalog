// Package catalog loads, checks, renders and tests the component catalog.
package catalog

import (
	"cmp"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
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
	APIVersion      string            `yaml:"apiVersion" json:"apiVersion"`
	Kind            string            `yaml:"kind" json:"kind"`
	Metadata        Metadata          `yaml:"metadata" json:"metadata"`
	Defaults        map[string]string `yaml:"defaults" json:"defaults"`
	Distributions   []string          `yaml:"distributions" json:"distributions"`
	KubeVersion     string            `yaml:"kubeVersion" json:"kubeVersion"`
	PersistsData    bool              `yaml:"persistsData" json:"persistsData,omitempty"`
	Requires        []Ref             `yaml:"requires" json:"requires,omitempty"`
	Inputs          []Input           `yaml:"inputs" json:"inputs,omitempty"`
	Releases        []Release         `yaml:"releases" json:"releases"`
	Provides        *Provides         `yaml:"provides" json:"provides,omitempty"`
	InternalCRDs    []string          `yaml:"internalCRDs" json:"internalCRDs,omitempty"`
	Resources       *Resources        `yaml:"resources" json:"-"`
	NetworkPolicies string            `yaml:"networkPolicies" json:"networkPolicies,omitempty"`
	Contributes     []Contribution    `yaml:"contributes" json:"contributes,omitempty"`
	Detection       map[string]any    `yaml:"detection" json:"detection"`
	Security        Security          `yaml:"security" json:"security"`
	Health          []Health          `yaml:"health" json:"health,omitempty"`
	Smoke           []Smoke           `yaml:"smoke" json:"smoke,omitempty"`
	Leftovers       []string          `yaml:"leftovers" json:"leftovers,omitempty"`

	Path string `yaml:"-" json:"-"` // relative to the repository root
}

// Metadata is a component's name and summary.
type Metadata struct {
	Name    string `yaml:"name" json:"name"`
	Summary string `yaml:"summary" json:"summary"`
	// Locales holds the summary in other languages, by language tag.
	Locales map[string]struct {
		Summary string `yaml:"summary" json:"summary"`
	} `yaml:"locales" json:"locales,omitempty"`
	License  string `yaml:"license" json:"license"`
	Homepage string `yaml:"homepage" json:"homepage"`
}

// Security is what a component's RBAC lets it do and what its namespaces must allow.
type Security struct {
	Summary string `yaml:"summary" json:"summary"`
	// PodSecurity is the Pod Security Standard level: restricted, baseline or privileged.
	PodSecurity string `yaml:"podSecurity" json:"podSecurity"`
}

// Ref names a component by ID, or a capability.
type Ref struct {
	Component  string `yaml:"component" json:"component,omitempty"`
	Capability string `yaml:"capability" json:"capability,omitempty"`
}

// ByID returns a ref to each component ID.
func ByID(ids ...string) []Ref {
	var out []Ref
	for _, id := range ids {
		out = append(out, Ref{Component: id})
	}
	return out
}

// ReleaseRef names a release of a component.
type ReleaseRef struct {
	Component string `yaml:"component" json:"component"`
	Release   string `yaml:"release" json:"release"`
}

// Provides is what a component offers other software in the cluster.
type Provides struct {
	Capability string    `yaml:"capability" json:"capability,omitempty"`
	Registry   *Registry `yaml:"registry" json:"registry,omitempty"`
}

// Capability is the capability the component provides, or "".
func (c *Component) Capability() string {
	if c.Provides == nil {
		return ""
	}
	return c.Provides.Capability
}

// Registry is an OCI registry in the cluster that nodes pull from.
type Registry struct {
	Host string    `yaml:"host" json:"host"`
	CA   SecretRef `yaml:"ca" json:"ca"`
}

// Resources is what a component needs beyond its measured idle use.
type Resources struct {
	Work []Work `yaml:"work"`
}

// Work is one kind of work a component does, such as a build.
type Work struct {
	Name string `yaml:"name"`
	// Requests and Limits are those of the pods doing one unit of it, for
	// pods that no render shows.
	Requests Quantities `yaml:"requests"`
	Limits   Quantities `yaml:"limits"`
	// Use is what one unit uses on top of the component's idle use.
	Use Quantities `yaml:"use"`
}

type Release struct {
	Name string `yaml:"name" json:"name"`
	// Type is helm, manifest (files at urls) or inline (manifests).
	Type          string            `yaml:"type" json:"type"`
	Namespace     string            `yaml:"namespace" json:"namespace"`
	ReleaseName   string            `yaml:"releaseName" json:"releaseName,omitempty"`
	SkipIfPresent []string          `yaml:"skipIfPresent" json:"skipIfPresent,omitempty"`
	Chart         string            `yaml:"chart" json:"chart,omitempty"`
	Repo          string            `yaml:"repo" json:"repo,omitempty"`
	URLs          []string          `yaml:"urls" json:"urls,omitempty"`
	Version       string            `yaml:"version" json:"version,omitempty"`
	ChartDigest   string            `yaml:"chartDigest" json:"chartDigest,omitempty"`
	URLDigests    []string          `yaml:"urlDigests" json:"urlDigests,omitempty"`
	Unpinned      []string          `yaml:"unpinnedImages" json:"unpinnedImages,omitempty"`
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
	Generate    *Generate  `yaml:"generate" json:"generate,omitempty"`
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
	Target ReleaseRef `yaml:"target" json:"target"`
	Values string     `yaml:"values" json:"values"`
}

// Health is a read-only check of an installed component: one kubectl wait
// that applies nothing, so it can run on a production cluster.
type Health struct {
	Name    string   `yaml:"name" json:"name"`
	Wait    []string `yaml:"wait" json:"wait"` // kubectl wait arguments, without --timeout
	Timeout string   `yaml:"timeout" json:"timeout"`
	Advice  string   `yaml:"advice" json:"advice"` // shown when the check fails
}

type Smoke struct {
	Name     string   `yaml:"name" json:"name"`
	Manifest string   `yaml:"manifest" json:"manifest,omitempty"`
	Wait     []string `yaml:"wait" json:"wait"`
	Timeout  string   `yaml:"timeout" json:"timeout,omitempty"`
}

// HelmName is the Helm release name.
func (r Release) HelmName() string { return cmp.Or(r.ReleaseName, r.Name) }

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

// Load reads components/<id>.yaml under root.
func Load(root string) (*Catalog, error) {
	nested, err := filepath.Glob(filepath.Join(root, "components", "*", "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(nested) > 0 {
		return nil, fmt.Errorf("%s: a component is components/<id>.yaml, with its capability in provides", nested[0])
	}
	files, err := filepath.Glob(filepath.Join(root, "components", "*.yaml"))
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
		c.ID = strings.TrimSuffix(filepath.Base(f), ".yaml")
		c.addDefaultAccounts()
		cat.Components = append(cat.Components, c)
	}
	slices.SortFunc(cat.Components, func(a, b *Component) int { return cmp.Compare(a.ID, b.ID) })
	return cat, nil
}

const defaultAccountPrefix = "default-account-"

// IsDefaultAccount reports whether the release is a namespace's default
// ServiceAccount, which every component of the namespace carries.
func (r Release) IsDefaultAccount() bool { return strings.HasPrefix(r.Name, defaultAccountPrefix) }

// addDefaultAccounts puts releases first that give each of the component's
// namespaces a default ServiceAccount mounting no token, so a Pod that names no
// account reaches nothing in the API. kube-system belongs to the distribution.
func (c *Component) addDefaultAccounts() {
	var added []Release
	seen := map[string]bool{"kube-system": true}
	for _, r := range c.Releases {
		if seen[r.Namespace] {
			continue
		}
		seen[r.Namespace] = true
		added = append(added, Release{
			Name:      defaultAccountPrefix + r.Namespace,
			Type:      "inline",
			Namespace: r.Namespace,
			Manifests: fmt.Sprintf("apiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: default\n  namespace: %s\nautomountServiceAccountToken: false\n", r.Namespace),
		})
	}
	c.Releases = append(added, c.Releases...)
}

// Get returns the component with the given ID.
func (cat *Catalog) Get(id string) (*Component, bool) {
	i := slices.IndexFunc(cat.Components, func(c *Component) bool { return c.ID == id })
	if i < 0 {
		return nil, false
	}
	return cat.Components[i], true
}

// provider resolves a reference for a profile: a component as is, a
// capability to its recommended provider, else an optional one.
func (cat *Catalog) provider(ref Ref, profile string) (*Component, error) {
	if ref.Component != "" {
		if c, ok := cat.Get(ref.Component); ok {
			return c, nil
		}
		return nil, fmt.Errorf("unknown component %q", ref.Component)
	}
	var optional *Component
	for _, c := range cat.Components {
		if c.Capability() != ref.Capability {
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
		return nil, fmt.Errorf("nothing provides %q on %s", ref.Capability, profile)
	}
	return optional, nil
}

// InstallOrder returns what refs name and their dependencies, dependencies first.
func (cat *Catalog) InstallOrder(refs []Ref, profile string) ([]*Component, error) {
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
		for _, ref := range c.Requires {
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
	for _, ref := range refs {
		c, err := cat.provider(ref, profile)
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
		if r.Type == "helm" {
			return r.Name
		}
	}
	return ""
}

// Generate is how an installer makes the value of an input it does not ask
// for. A random value has Bytes and Encoding. A reader does not fill in the
// value of an input whose Type it does not know.
type Generate struct {
	Type     string `yaml:"type" json:"type"`
	Bytes    int    `yaml:"bytes" json:"bytes,omitempty"`
	Encoding string `yaml:"encoding" json:"encoding,omitempty"`
}

// Types of Generate.
const (
	GenerateRandom    = "random"
	GenerateClusterIP = "clusterIP"
)

// Random returns g.Bytes random bytes in g.Encoding, hex or base64url
// without padding.
func (g Generate) Random() string {
	b := make([]byte, g.Bytes)
	rand.Read(b)
	if g.Encoding == "hex" {
		return hex.EncodeToString(b)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Generates reports whether an installer generates the value of in as t.
func (in Input) Generates(t string) bool { return in.Generate != nil && in.Generate.Type == t }

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
