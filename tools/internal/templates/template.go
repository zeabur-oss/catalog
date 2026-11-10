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

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"go.yaml.in/yaml/v3"
)

type Template struct {
	ID         string        `yaml:"-" json:"id"`
	APIVersion string        `yaml:"apiVersion" json:"apiVersion"`
	Kind       string        `yaml:"kind" json:"kind"`
	Metadata   Metadata      `yaml:"metadata" json:"metadata"`
	Requires   []catalog.Ref `yaml:"requires" json:"requires,omitempty"`
	Services   []Service     `yaml:"services" json:"services"`
	Inputs     []Input       `yaml:"inputs" json:"inputs,omitempty"`
	Security   Security      `yaml:"security" json:"security"`
	Smoke      []Smoke       `yaml:"smoke" json:"smoke"`

	Dir string `yaml:"-" json:"-"` // holds template.yaml and manifests.yaml
	raw any
}

type Metadata struct {
	catalog.Metadata `yaml:",inline"`
	Version          string `yaml:"version" json:"version"`
}

type Security struct {
	Summary string `yaml:"summary" json:"summary"`
	Locales map[string]struct {
		Summary string `yaml:"summary" json:"summary"`
	} `yaml:"locales" json:"locales,omitempty"`
	Risks []Risk `yaml:"risks" json:"risks"`
	// Unauthenticated names the services that have no sign-in or key of their
	// own. Each needs a NetworkPolicy that limits who can connect to it.
	Unauthenticated []string `yaml:"unauthenticated" json:"unauthenticated,omitempty"`
}

type Risk struct {
	Risk       string `yaml:"risk" json:"risk"`
	Mitigation string `yaml:"mitigation" json:"mitigation"`
	Locales    map[string]struct {
		Risk       string `yaml:"risk" json:"risk"`
		Mitigation string `yaml:"mitigation" json:"mitigation"`
	} `yaml:"locales" json:"locales,omitempty"`
}

type Service struct {
	Name        string                       `yaml:"name" json:"name"`
	PodSecurity string                       `yaml:"podSecurity" json:"podSecurity"`
	Ports       []Port                       `yaml:"ports" json:"ports,omitempty"`
	Resources   map[string]map[string]string `yaml:"resources" json:"resources"`
	Domains     []string                     `yaml:"domains" json:"domains,omitempty"`
}

type Port struct {
	Name string `yaml:"name" json:"name"`
	Port int    `yaml:"port" json:"port"`
}

type Input struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Label       string `yaml:"label" json:"label,omitempty"`
	Locales     map[string]struct {
		Label       string `yaml:"label" json:"label"`
		Description string `yaml:"description" json:"description"`
	} `yaml:"locales" json:"locales,omitempty"`
	Generate *catalog.Generate `yaml:"generate" json:"generate,omitempty"`
	Required bool              `yaml:"required" json:"required,omitempty"`
	// Default is the value of an input that is not given.
	Default string `yaml:"default" json:"default,omitempty"`
	Secret  *struct {
		Name string `yaml:"name" json:"name"`
		Key  string `yaml:"key" json:"key"`
	} `yaml:"secret" json:"secret,omitempty"`
	Validation struct {
		Pattern string `yaml:"pattern" json:"pattern"`
	} `yaml:"validation" json:"validation"`
}

type Smoke struct {
	Name      string `yaml:"name" json:"name"`
	Resource  string `yaml:"resource" json:"resource"`
	Condition string `yaml:"condition" json:"condition"`
	Timeout   string `yaml:"timeout" json:"timeout"`
	HTTP      *struct {
		Service  string `yaml:"service" json:"service"`
		Port     int    `yaml:"port" json:"port"`
		Path     string `yaml:"path" json:"path"`
		Status   int    `yaml:"status" json:"status"`
		Contains string `yaml:"contains" json:"contains,omitempty"`
	} `yaml:"http" json:"http,omitempty"`
	SQL *struct {
		Cluster  string `yaml:"cluster" json:"cluster"`
		Database string `yaml:"database" json:"database"`
		Query    string `yaml:"query" json:"query"`
		Expect   string `yaml:"expect" json:"expect"`
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
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	schemaPath, err := filepath.Abs(filepath.Join(root, "schema", "v1", "template.schema.json"))
	if err != nil {
		return err
	}
	schema, err := compiler.Compile(schemaPath)
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
