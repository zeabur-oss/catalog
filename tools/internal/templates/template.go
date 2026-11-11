// Package templates loads, checks, renders and tests the application
// templates a project member installs into one namespace.
package templates

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"text/template"
	"text/template/parse"

	gen "github.com/invopop/jsonschema"
	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"go.yaml.in/yaml/v4"
)

// Template is one application, at templates/<slug>/template.yaml with its objects in
// manifests.yaml next to it. A project member installs it into one namespace.
type Template struct {
	ID         string   `yaml:"-" json:"id"`
	APIVersion string   `yaml:"apiVersion" json:"apiVersion" jsonschema:"required"`
	Kind       string   `yaml:"kind" json:"kind" jsonschema:"required"`
	Metadata   Metadata `yaml:"metadata" json:"metadata" jsonschema:"required"`
	// Components the cluster must have.
	Requires []catalog.Ref `yaml:"requires" json:"requires,omitempty" jsonschema:"uniqueItems=true"`
	// The workloads in manifests.yaml, for people and for the Pod Security profile of each.
	Services []Service `yaml:"services" json:"services" jsonschema:"required,minItems=1"`
	Inputs   []Input   `yaml:"inputs" json:"inputs,omitempty"`
	Security Security  `yaml:"security" json:"security" jsonschema:"required"`
	// e2e checks after install, in the template's namespace.
	Smoke []Smoke `yaml:"smoke" json:"smoke" jsonschema:"required,minItems=1"`

	Dir string `yaml:"-" json:"-"` // holds template.yaml and manifests.yaml
	raw any
}

func (Template) JSONSchemaExtend(s *gen.Schema) {
	s.Properties.Set("apiVersion", &gen.Schema{Const: "catalog.zeabur.com/v1"})
	s.Properties.Set("kind", &gen.Schema{Const: "Template"})
}

// Metadata is a template's name, summary and version.
type Metadata struct {
	catalog.Metadata `yaml:",inline"`
	// The application version: the tag of its main image. Objects get it as the label
	// app.kubernetes.io/version.
	Version string `yaml:"version" json:"version" jsonschema:"required"`
}

func (Metadata) JSONSchemaExtend(s *gen.Schema) {
	nonEmpty(s, "name", "Name for people.")
	nonEmpty(s, "license", "SPDX expression for the application, its images and charts.")
	summary, _ := s.Properties.Get("summary")
	summary.Description = "What the application is, in one plain sentence for people choosing a template."
	homepage, _ := s.Properties.Get("homepage")
	homepage.Pattern = "^https?://"
}

// nonEmpty makes property name a string of at least one character.
func nonEmpty(s *gen.Schema, name, description string) {
	s.Properties.Set(name, &gen.Schema{Type: "string", MinLength: new(uint64(1)), Description: description})
}

// Security is what a template exposes and how its risks are mitigated.
type Security struct {
	Summary string                 `yaml:"summary" json:"summary" jsonschema:"required,minLength=1"`
	Locales catalog.SummaryLocales `yaml:"locales" json:"locales,omitempty"`
	Risks   []Risk                 `yaml:"risks" json:"risks" jsonschema:"required"`
	// Services with no sign-in or key of their own. Each needs an ingress NetworkPolicy; the risks
	// say why.
	Unauthenticated []string `yaml:"unauthenticated" json:"unauthenticated,omitempty" jsonschema:"uniqueItems=true"`
}

// Risk is a risk of a template and its mitigation.
type Risk struct {
	Risk       string      `yaml:"risk" json:"risk" jsonschema:"required,minLength=1"`
	Mitigation string      `yaml:"mitigation" json:"mitigation" jsonschema:"required,minLength=1"`
	Locales    RiskLocales `yaml:"locales" json:"locales,omitempty" jsonschema:"required"`
}

// RiskLocales is the risk and mitigation in other languages, by BCP 47 tag. config/locales.yaml
// lists the tags each needs.
type RiskLocales map[catalog.Locale]struct {
	Risk       string `yaml:"risk" json:"risk" jsonschema:"required,minLength=1"`
	Mitigation string `yaml:"mitigation" json:"mitigation" jsonschema:"required,minLength=1"`
}

func (RiskLocales) JSONSchemaExtend(s *gen.Schema) { catalog.LocaleKeys(s) }

// Service is a workload in manifests.yaml.
type Service struct {
	Name string `yaml:"name" json:"name" jsonschema:"required,pattern=^[a-z][a-z0-9-]*$,maxLength=53"`
	// The Pod Security Standard level its pods meet.
	PodSecurity string           `yaml:"podSecurity" json:"podSecurity" jsonschema:"required,enum=restricted,enum=baseline"`
	Ports       []Port           `yaml:"ports" json:"ports,omitempty"`
	Resources   ServiceResources `yaml:"resources" json:"resources" jsonschema:"required"`
	// Public hostnames; rendering then needs a domain and a ClusterIssuer.
	Domains []string `yaml:"domains" json:"domains,omitempty" jsonschema:"minLength=1"`
}

// ServiceResources is the requests and limits of a service, by level and resource name.
type ServiceResources map[string]map[string]string

func (ServiceResources) JSONSchema() *gen.Schema {
	values := gen.NewProperties()
	values.Set("cpu", &gen.Schema{Type: "string", Pattern: `^[0-9]+(\.[0-9]+)?m?$`})
	values.Set("memory", &gen.Schema{Type: "string", Pattern: "^[1-9][0-9]*(Ki|Mi|Gi|Ti|K|M|G|T)?$"})
	v := &gen.Schema{Type: "object", Properties: values, Required: []string{"cpu", "memory"}, AdditionalProperties: gen.FalseSchema}
	levels := gen.NewProperties()
	levels.Set("requests", v)
	levels.Set("limits", v)
	return &gen.Schema{Type: "object", Properties: levels, Required: []string{"requests", "limits"}, AdditionalProperties: gen.FalseSchema}
}

// Port is a named port of a service.
type Port struct {
	Name string `yaml:"name" json:"name" jsonschema:"required,minLength=1"`
	Port int    `yaml:"port" json:"port" jsonschema:"required,minimum=1,maximum=65535"`
}

// Input is a value the person installing gives (required) or the installer generates into a
// Secret (generate).
type Input struct {
	// camelCase.
	Name        string `yaml:"name" json:"name" jsonschema:"required,pattern=^[a-z][A-Za-z0-9]*$"`
	Description string `yaml:"description" json:"description" jsonschema:"required,minLength=1"`
	// A short name for the field, in English.
	Label    string            `yaml:"label" json:"label,omitempty" jsonschema:"maxLength=40"`
	Locales  InputLocales      `yaml:"locales" json:"locales,omitempty"`
	Generate *catalog.Generate `yaml:"generate" json:"generate,omitempty"`
	Required bool              `yaml:"required" json:"required,omitempty"`
	// The value when a deploy does not give the input.
	Default string `yaml:"default" json:"default,omitempty"`
	Secret  *struct {
		Name string `yaml:"name" json:"name" jsonschema:"required,pattern=^[a-z0-9][a-z0-9.-]*$"`
		Key  string `yaml:"key" json:"key" jsonschema:"required,pattern=^[A-Za-z0-9_.-]+$"`
	} `yaml:"secret" json:"secret,omitempty"`
	Validation struct {
		// Regular expression the whole value matches.
		Pattern string `yaml:"pattern" json:"pattern" jsonschema:"required,minLength=1"`
	} `yaml:"validation" json:"validation" jsonschema:"required"`
}

func (Input) JSONSchemaExtend(s *gen.Schema) {
	s.OneOf = []*gen.Schema{
		{Required: []string{"generate", "secret"},
			Not: &gen.Schema{AnyOf: []*gen.Schema{{Required: []string{"required"}}, {Required: []string{"default"}}}}},
		{Description: "A person gives the value, so the field has a label in every language.",
			Required: []string{"required", "label", "locales"},
			Not:      &gen.Schema{AnyOf: []*gen.Schema{{Required: []string{"generate"}}, {Required: []string{"secret"}}}}},
	}
}

// InputLocales is the label and description in other languages, by BCP 47 tag.
// config/locales.yaml lists the tags each needs.
type InputLocales map[catalog.Locale]struct {
	Label       string `yaml:"label" json:"label" jsonschema:"required,maxLength=40"`
	Description string `yaml:"description" json:"description" jsonschema:"required,minLength=1"`
}

func (InputLocales) JSONSchemaExtend(s *gen.Schema) { catalog.LocaleKeys(s) }

// Smoke is an e2e check after install.
type Smoke struct {
	Name      string `yaml:"name" json:"name" jsonschema:"required,minLength=1"`
	Resource  string `yaml:"resource" json:"resource" jsonschema:"required,pattern=^[a-zA-Z0-9.-]+/[a-z0-9][a-z0-9.-]*$"`
	Condition string `yaml:"condition" json:"condition" jsonschema:"required,pattern=^(condition=[A-Za-z][A-Za-z0-9]*(=(True|False|Unknown))?|jsonpath=.+)$"`
	Timeout   string `yaml:"timeout" json:"timeout" jsonschema:"required,pattern=^[1-9][0-9]*(s|m)$"`
	// GET through a port-forward to the Service.
	HTTP *struct {
		Service  string `yaml:"service" json:"service" jsonschema:"required,pattern=^[a-z0-9][a-z0-9-]*$"`
		Port     int    `yaml:"port" json:"port" jsonschema:"required,minimum=1,maximum=65535"`
		Path     string `yaml:"path" json:"path" jsonschema:"required,pattern=^/"`
		Status   int    `yaml:"status" json:"status" jsonschema:"required,minimum=100,maximum=599"`
		Contains string `yaml:"contains" json:"contains,omitempty"`
	} `yaml:"http" json:"http,omitempty"`
	// One read-only SELECT on the CloudNativePG primary.
	SQL *struct {
		Cluster  string `yaml:"cluster" json:"cluster" jsonschema:"required,pattern=^[a-z][a-z0-9-]*$"`
		Database string `yaml:"database" json:"database" jsonschema:"required,pattern=^[A-Za-z][A-Za-z0-9_]*$"`
		Query    string `yaml:"query" json:"query" jsonschema:"required,pattern=^[Ss][Ee][Ll][Ee][Cc][Tt] "`
		Expect   string `yaml:"expect" json:"expect" jsonschema:"required"`
	} `yaml:"sql" json:"sql,omitempty"`
}

// Load reads templates/<id>/template.yaml under root, sorted by ID.
func Load(root string) ([]*Template, error) {
	files, err := filepath.Glob(filepath.Join(root, "templates", "*", "template.yaml"))
	if err != nil {
		return nil, err
	}
	var out []*Template
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(root, f)
		t := &Template{Dir: filepath.Dir(f), ID: filepath.Base(filepath.Dir(f))}
		if err := yaml.Unmarshal(data, &t.raw); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if err := yaml.Unmarshal(data, t); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		out = append(out, t)
	}
	return out, nil
}

// Get returns the template with the given ID.
func Get(all []*Template, id string) (*Template, error) {
	for _, t := range all {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("unknown template %q", id)
}

// The e2e namespace templates-e2e-<id> must be a DNS label.
var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Check validates every template without a cluster: schema, ID, services,
// inputs, manifests syntax and that the catalog provides what each requires.
func Check(root string, cat *catalog.Catalog, all []*Template) error {
	var errs []error
	schema, err := catalog.CompileSchema(filepath.Join(root, "schema", "v1"), "template.schema.json")
	if err != nil {
		return err
	}
	locales, err := catalog.ReadLocales(root)
	if err != nil {
		return err
	}
	for _, t := range all {
		fail := func(format string, a ...any) {
			errs = append(errs, fmt.Errorf("templates/%s: %s", t.ID, fmt.Sprintf(format, a...)))
		}
		if err := schema.Validate(t.raw); err != nil {
			fail("%v", err)
			continue
		}
		for _, p := range catalog.MissingLocales(t.raw, locales) {
			fail("%s", p)
		}
		if !idPattern.MatchString(t.ID) || len(t.ID) > 49 {
			fail("directory name must be a DNS label of at most 49 characters")
		}
		if _, ok := cat.Get(t.ID); ok {
			fail("components/%s.yaml has the same ID", t.ID)
		}
		names := map[string]bool{}
		for _, s := range t.Services {
			if names[s.Name] {
				fail("duplicate service %s", s.Name)
			}
			names[s.Name] = true
		}
		if _, err := validateInputs(t.Inputs); err != nil {
			fail("%v", err)
		}
		if data, err := os.ReadFile(filepath.Join(t.Dir, "manifests.yaml")); err != nil {
			fail("%v", err)
		} else if err := checkActions(data, t.Inputs); err != nil {
			fail("manifests.yaml: %v", err)
		} else if src, err := os.ReadFile(filepath.Join(t.Dir, "template.yaml")); err != nil {
			fail("%v", err)
		} else if err := CheckVersion(src, data, t.Metadata.Version); err != nil {
			fail("%v", err)
		}
		for _, p := range catalog.Profiles {
			if _, err := cat.InstallOrder(t.Requires, p); err != nil {
				fail("requires on %s: %v", p, err)
			}
		}
	}
	return errors.Join(errs...)
}

// versionImage is the line above metadata.version that names the image whose tag it is, so that
// Renovate moves the version with that image.
var versionImage = regexp.MustCompile(`(?m)^  # renovate: depName=(\S+)\n  version: "?([^"\s]+)"?$`)

// CheckVersion reports whether metadata.version names, on the line above it, an image of the
// manifests that is pinned at that tag with a digest.
func CheckVersion(templateYAML, manifests []byte, version string) error {
	m := versionImage.FindSubmatch(templateYAML)
	if m == nil || string(m[2]) != version {
		return errors.New(`metadata.version needs "# renovate: depName=<image>" on the line above it`)
	}
	image := regexp.MustCompile(`(^|[\s"'])` + regexp.QuoteMeta(string(m[1])+":"+version+"@sha256:"))
	if !labelValue.MatchString(version) || !image.Match(manifests) {
		return fmt.Errorf("metadata.version must be the tag of %s in manifests.yaml, pinned with a digest", m[1])
	}
	return nil
}

// RequiresCertificates reports whether the template requires a component
// that provides certificates.
func (t *Template) RequiresCertificates(cat *catalog.Catalog) bool {
	for _, r := range t.Requires {
		if r.Capability == "certificates" {
			return true
		}
		if c, ok := cat.Get(r.Component); ok && c.Capability() == "certificates" {
			return true
		}
	}
	return false
}

func parseManifests(manifests []byte) (*template.Template, error) {
	return template.New("manifests").Option("missingkey=error").Parse(string(manifests))
}

var trimMarker = regexp.MustCompile(`\{\{-[ \t\r\n]|[ \t\r\n]-\}\}`)

// checkActions accepts only field substitution: {{ .Namespace }}, {{ .Domain }},
// {{ .ClusterIssuer }} and {{ .Inputs.<name> }} of an input that is not generated.
func checkActions(manifests []byte, inputs []Input) error {
	if trimMarker.Match(manifests) {
		return errors.New("trim markers are not allowed")
	}
	t, err := parseManifests(manifests)
	if err != nil {
		return err
	}
	given := map[string]bool{}
	for _, in := range inputs {
		if in.Generate == nil {
			given[in.Name] = true
		}
	}
	for _, n := range t.Tree.Root.Nodes {
		switch n := n.(type) {
		case *parse.TextNode:
		case *parse.ActionNode:
			if f := actionField(n); len(f) == 0 || !(len(f) == 1 && slices.Contains([]string{"Namespace", "Domain", "ClusterIssuer"}, f[0]) || len(f) == 2 && f[0] == "Inputs" && given[f[1]]) {
				return fmt.Errorf("only {{ .Namespace }}, {{ .Domain }}, {{ .ClusterIssuer }} and {{ .Inputs.<name> }} of a given input are allowed, not %s", n)
			}
		default:
			return fmt.Errorf("only field substitution is allowed, not %s", n)
		}
	}
	return nil
}

// actionField returns the path of the field that a plain {{ .A.B }} action
// reads, or nil for any other action.
func actionField(n *parse.ActionNode) []string {
	p := n.Pipe
	if len(p.Decl) > 0 || len(p.Cmds) != 1 || len(p.Cmds[0].Args) != 1 {
		return nil
	}
	if f, ok := p.Cmds[0].Args[0].(*parse.FieldNode); ok {
		return f.Ident
	}
	return nil
}
