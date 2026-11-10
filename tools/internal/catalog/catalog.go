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

	gen "github.com/invopop/jsonschema"
	"go.yaml.in/yaml/v3"
)

// Profiles every component describes.
var Profiles = []string{"single-node", "multi-node"}

// Component is one upstream, at components/<id>.yaml. A cluster administrator installs it.
type Component struct {
	ID         string   `yaml:"-" json:"-"`
	APIVersion string   `yaml:"apiVersion" jsonschema:"required"`
	Kind       string   `yaml:"kind" jsonschema:"required"`
	Metadata   Metadata `yaml:"metadata" jsonschema:"required"`
	// Per profile: recommended (on by default), optional (off by default) or unsupported.
	Defaults Modes `yaml:"defaults" jsonschema:"required"`
	// Distributions it runs on and e2e tests it on, of k3s, rke2 and kubesolo. kind tests it
	// regardless. A reader ignores a value it does not know.
	Distributions []string `yaml:"distributions" jsonschema:"required,minItems=1,uniqueItems=true,enum=k3s,enum=rke2,enum=kubesolo"`
	// The Kubernetes versions it supports, as a SemVer range like Helm's Chart.yaml kubeVersion:
	// the strictest kubeVersion of its charts, else the oldest version that config/kubernetes.yaml
	// renders. Every version that config/kubernetes.yaml renders or tests on must be in it.
	KubeVersion string `yaml:"kubeVersion" jsonschema:"required,minLength=1"`
	// Keeps state in volumes; uninstalling loses data.
	PersistsData bool `yaml:"persistsData"`
	// Components installed first.
	Requires []Ref `yaml:"requires" jsonschema:"uniqueItems=true"`
	// Values the installer asks for or generates.
	Inputs []Input `yaml:"inputs"`
	// Installed in order.
	Releases []Release `yaml:"releases" jsonschema:"required,minItems=1"`
	// What it offers other software in the cluster.
	Provides *Provides `yaml:"provides"`
	// CRDs of detection.crds whose objects the component writes itself, so they don't mean anyone
	// uses it.
	InternalCRDs []string   `yaml:"internalCRDs"`
	Resources    *Resources `yaml:"resources"`
	// Standard NetworkPolicy objects in its namespaces, for installers that block incoming traffic
	// to each namespace by default: applied before the releases, they admit the traffic the
	// component needs from elsewhere.
	NetworkPolicies string `yaml:"networkPolicies"`
	// Values this component adds to another component's release.
	Contributes []Contribution `yaml:"contributes"`
	Detection   Detection      `yaml:"detection" jsonschema:"required"`
	Security    Security       `yaml:"security" jsonschema:"required"`
	// Read-only checks that a tool such as zeabur-installer doctor runs on an installed cluster.
	// Each is one `kubectl wait` and applies nothing. e2e runs them after install, before smoke.
	Health []Health `yaml:"health"`
	// Extra e2e checks after install. They may apply objects in catalog-smoke, so only e2e runs
	// them. Read-only checks go in health.
	Smoke []Smoke `yaml:"smoke"`
	// What the upstream keeps on uninstall: a kind (customresourcedefinitions) or
	// <resource>/<name>.
	Leftovers []string `yaml:"leftovers"`

	Path string `yaml:"-" json:"-"` // relative to the repository root
}

func (Component) JSONSchemaExtend(s *gen.Schema) {
	constant(s, "apiVersion", "catalog.zeabur.com/v1")
	constant(s, "kind", "Component")
}

// constant sets the schema of property name to the value v.
func constant(s *gen.Schema, name string, v any) {
	s.Properties.Set(name, &gen.Schema{Const: v})
}

// Modes is the mode of a component on each profile.
type Modes map[string]string

func (Modes) JSONSchema() *gen.Schema {
	return profileMap(&gen.Schema{Enum: []any{"recommended", "optional", "unsupported"}}, true)
}

// profileMap is an object with a value of schema v for each profile.
func profileMap(v *gen.Schema, required bool) *gen.Schema {
	s := &gen.Schema{Type: "object", Properties: gen.NewProperties(), AdditionalProperties: gen.FalseSchema}
	for _, p := range Profiles {
		s.Properties.Set(p, v)
	}
	if required {
		s.Required = Profiles
	}
	return s
}

// Metadata is a component's name and summary.
type Metadata struct {
	// Name for people.
	Name string `yaml:"name" jsonschema:"required"`
	// What it gives the cluster, in one plain sentence for people choosing components.
	Summary string         `yaml:"summary" jsonschema:"required,pattern=^[^\\n]+$"`
	Locales SummaryLocales `yaml:"locales" jsonschema:"required"`
	// SPDX expression; every identifier must be OSI-approved, or listed under optInOnly in
	// config/licenses.yaml with no profile recommending the component.
	License  string `yaml:"license" jsonschema:"required"`
	Homepage string `yaml:"homepage" jsonschema:"required,format=uri"`
}

// Security is what a component's RBAC lets it do and what its namespaces must allow.
type Security struct {
	// What its RBAC lets it do.
	Summary string `yaml:"summary" jsonschema:"required"`
	// The Pod Security Standard level its namespaces need.
	PodSecurity string `yaml:"podSecurity" jsonschema:"required,enum=restricted,enum=baseline,enum=privileged"`
}

// Ref is a component by id, or a capability: its recommended upstream on the profile, else an
// optional one.
type Ref struct {
	Component  string `yaml:"component" json:"component,omitempty" jsonschema:"pattern=^[a-z0-9]+(-[a-z0-9]+)*$"`
	Capability string `yaml:"capability" json:"capability,omitempty" jsonschema:"pattern=^[a-z0-9-]+$"`
}

func (Ref) JSONSchemaExtend(s *gen.Schema) {
	s.OneOf = []*gen.Schema{{Required: []string{"component"}}, {Required: []string{"capability"}}}
}

// ByID returns a ref to each component ID.
func ByID(ids ...string) []Ref {
	var out []Ref
	for _, id := range ids {
		out = append(out, Ref{Component: id})
	}
	return out
}

// ReleaseRef is the release of another component that receives the values.
type ReleaseRef struct {
	Component string `yaml:"component" jsonschema:"required,pattern=^[a-z0-9]+(-[a-z0-9]+)*$"`
	Release   string `yaml:"release" jsonschema:"required"`
}

// Provides is what a component offers other software in the cluster.
type Provides struct {
	// The capability it offers, such as logs. A requires entry {capability: <name>} installs one
	// component that offers it.
	Capability string    `yaml:"capability" jsonschema:"pattern=^[a-z0-9-]+$"`
	Registry   *Registry `yaml:"registry"`
}

func (Provides) JSONSchemaExtend(s *gen.Schema) { s.MinProperties = new(uint64(1)) }

// Capability is the capability the component provides, or "".
func (c *Component) Capability() string {
	if c.Provides == nil {
		return ""
	}
	return c.Provides.Capability
}

// Registry is an OCI registry in the cluster that nodes pull from.
type Registry struct {
	// host:port that images name.
	Host string `yaml:"host" jsonschema:"required,pattern=^[a-z0-9.-]+:[0-9]+$"`
	// The CA that signs its serving certificate.
	CA SecretRef `yaml:"ca" jsonschema:"required"`
}

// Resources is what a component needs beyond its measured idle use.
type Resources struct {
	// Each kind of work the component does, such as a build. Names are unique.
	Work []Work `yaml:"work" jsonschema:"required,minItems=1"`
}

// Work is one kind of work a component does, such as a build.
type Work struct {
	Name string `yaml:"name" jsonschema:"required,pattern=^[a-z0-9-]+$"`
	// Resource requests of each pod doing one unit of work, for pods that no render shows, such as
	// a build pod.
	Requests Quantities `yaml:"requests"`
	// Resource limits of each pod doing one unit of work.
	Limits Quantities `yaml:"limits"`
	// What one unit of work uses on top of the component's idle use.
	Use Quantities `yaml:"use"`
}

func (Work) JSONSchemaExtend(s *gen.Schema) { s.AnyOf = anyRequired("requests", "limits", "use").AnyOf }

// Release is one install step of a component.
type Release struct {
	Name string `yaml:"name" jsonschema:"required,pattern=^[a-z0-9-]+$"`
	// helm: a chart. manifest: files downloaded from urls. inline: the objects in manifests.
	Type      string `yaml:"type" jsonschema:"required,enum=helm,enum=manifest,enum=inline"`
	Namespace string `yaml:"namespace" jsonschema:"required"`
	// Helm release name; defaults to name.
	ReleaseName string `yaml:"releaseName"`
	// Skip when these CRDs already exist.
	SkipIfPresent []string `yaml:"skipIfPresent"`
	// oci://registry/path/chart, or a chart name with repo.
	Chart string `yaml:"chart"`
	// HTTP chart repository.
	Repo string `yaml:"repo" jsonschema:"pattern=^https://"`
	// Manifest URLs; {version} is replaced.
	URLs    []string `yaml:"urls" jsonschema:"pattern=\\{version\\}"`
	Version string   `yaml:"version"`
	// The chart at version: for an oci:// chart, its OCI manifest digest; for a chart in an HTTP
	// repository, the SHA-256 of the chart archive, as the repository's index.yaml records it.
	ChartDigest string `yaml:"chartDigest" jsonschema:"pattern=^sha256:[a-f0-9]{64}$"`
	// The SHA-256 of each file in urls, in the same order.
	URLDigests []string `yaml:"urlDigests" jsonschema:"pattern=^sha256:[a-f0-9]{64}$"`
	// Images, without tag, that the chart cannot pin by digest. A comment says why.
	Unpinned []string `yaml:"unpinnedImages"`
	// Objects applied as written.
	Manifests string `yaml:"manifests"`
	// Values for every profile: Helm values (YAML). Every value needs a comment saying why.
	Values string `yaml:"values"`
	// Per-profile values merged over values (maps merge, lists replace).
	Profiles map[string]string `yaml:"profiles"`
}

func (Release) JSONSchemaExtend(s *gen.Schema) {
	s.Properties.Set("profiles", profileMap(&gen.Schema{Type: "string"}, false))
	of := func(t string, required []string, forbidden ...string) *gen.Schema {
		s := typed(t)
		s.Required, s.Not = required, anyRequired(forbidden...)
		return s
	}
	s.OneOf = []*gen.Schema{
		of("helm", []string{"chart", "version", "chartDigest"}, "urls", "manifests"),
		of("manifest", []string{"urls", "version", "urlDigests"}, "chart", "manifests"),
		of("inline", []string{"manifests"}, "chart", "urls"),
	}
	s.DependentRequired = map[string][]string{"repo": {"chart"}, "values": {"chart"}, "profiles": {"chart"},
		"chartDigest": {"chart"}, "urlDigests": {"urls"}}
}

// Input is a value the installer asks for or generates.
type Input struct {
	// camelCase.
	Name        string `yaml:"name" jsonschema:"required,pattern=^[a-z][A-Za-z0-9]*$"`
	Description string `yaml:"description" jsonschema:"required"`
	// Release whose values receive the input; defaults to the first chart release.
	Release string `yaml:"release"`
	// Dotted values path; numbers index lists.
	Path string `yaml:"path"`
	// Store the input in this Secret, created before install.
	Secret   *SecretRef `yaml:"secret"`
	Generate *Generate  `yaml:"generate"`
	Required bool       `yaml:"required"`
	Default  any        `yaml:"default"`
	// Value CI installs with.
	TestValue any `yaml:"testValue"`
}

func (Input) JSONSchemaExtend(s *gen.Schema) {
	s.OneOf = []*gen.Schema{{Required: []string{"path"}}, {Required: []string{"secret"}}}
}

// SecretRef is a key of a Secret.
type SecretRef struct {
	Name      string `yaml:"name" jsonschema:"required"`
	Key       string `yaml:"key" jsonschema:"required"`
	Namespace string `yaml:"namespace" jsonschema:"required"`
}

// Contribution is values that a component adds to a release of another component.
type Contribution struct {
	Target ReleaseRef `yaml:"target" jsonschema:"required"`
	// Helm values (YAML). Every value needs a comment saying why.
	Values string `yaml:"values" jsonschema:"required"`
}

// Detection is how to recognise an existing installation.
type Detection struct {
	CRDs               []string `yaml:"crds"`
	APIServices        []string `yaml:"apiServices"`
	GatewayControllers []string `yaml:"gatewayControllers"`
	IngressControllers []string `yaml:"ingressControllers"`
	// CSI drivers of the PersistentVolumes it provides.
	CSIDrivers []string `yaml:"csiDrivers"`
	HelmCharts []string `yaml:"helmCharts"`
	// Add-ons of a distribution that install the same upstream, such as k3s's packaged Traefik.
	Addons    []Addon            `yaml:"addons"`
	Workloads []DetectedWorkload `yaml:"workloads"`
}

func (Detection) JSONSchemaExtend(s *gen.Schema) { s.MinProperties = new(uint64(1)) }

// Addon is an add-on of a distribution.
type Addon struct {
	// A reader ignores an entry of a distribution it does not know.
	Distribution string `yaml:"distribution" jsonschema:"required"`
	Name         string `yaml:"name" jsonschema:"required"`
}

// DetectedWorkload is a workload whose labels show the upstream.
type DetectedWorkload struct {
	Kind   string            `yaml:"kind" jsonschema:"required,enum=Deployment,enum=StatefulSet,enum=DaemonSet"`
	Labels map[string]string `yaml:"labels" jsonschema:"required"`
}

// Health is a read-only check of an installed component: one kubectl wait
// that applies nothing, so it can run on a production cluster.
type Health struct {
	Name string `yaml:"name" jsonschema:"required,pattern=^[a-z0-9-]+$"`
	// `kubectl wait` arguments, one per item, with a --for condition and without --timeout.
	Wait    []string `yaml:"wait" jsonschema:"required,minItems=2,minLength=1"`
	Timeout string   `yaml:"timeout" jsonschema:"required,pattern=^[0-9]+(s|m)$"`
	// What to do when the check fails, in English. An installer shows it to the reader.
	Advice string `yaml:"advice" jsonschema:"required,minLength=1"`
}

func (Health) JSONSchemaExtend(s *gen.Schema) {
	wait, _ := s.Properties.Get("wait")
	wait.Contains = &gen.Schema{Pattern: "^--for="}
	wait.Not = &gen.Schema{Contains: &gen.Schema{Pattern: "^--timeout"}}
}

// Smoke is an e2e check after install.
type Smoke struct {
	Name string `yaml:"name" jsonschema:"required"`
	// Objects to apply first, in namespace catalog-smoke.
	Manifest string `yaml:"manifest"`
	// `kubectl wait` arguments.
	Wait    []string `yaml:"wait" jsonschema:"required"`
	Timeout string   `yaml:"timeout" jsonschema:"default=180s"`
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

// Generate is how the installer makes the value instead of asking. A reader does not fill in an
// input whose type it does not know.
type Generate struct {
	// random: bytes random bytes in encoding. clusterIP: a free address in the cluster's Service
	// CIDR, for a Service's fixed ClusterIP. A template generates random values only.
	Type string `yaml:"type" jsonschema:"required"`
	// Random bytes to generate.
	Bytes int `yaml:"bytes" jsonschema:"minimum=12,maximum=64"`
	// hex, or base64url without padding.
	Encoding string `yaml:"encoding" jsonschema:"enum=hex,enum=base64url"`
}

func (Generate) JSONSchemaExtend(s *gen.Schema) {
	random, clusterIP := typed(GenerateRandom), typed(GenerateClusterIP)
	random.Required = []string{"bytes", "encoding"}
	clusterIP.Not = anyRequired("bytes", "encoding")
	s.OneOf = []*gen.Schema{random, clusterIP}
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
