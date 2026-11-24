// Package templates loads, checks, renders and tests the application
// templates a project member installs into one namespace.
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"go.yaml.in/yaml/v4"
)

// Template is one application, at templates/<slug>/template.yaml with its objects in
// manifests.yaml next to it. A project member installs it into one namespace.
type Template struct {
	ID         string   `yaml:"-" json:"id"`
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	// Components the cluster must have.
	Requires []catalog.Ref `yaml:"requires" json:"requires,omitempty"`
	// The workloads in manifests.yaml, for people and for the Pod Security profile of each.
	Services []Service `yaml:"services" json:"services"`
	Inputs   []Input   `yaml:"inputs" json:"inputs,omitempty"`
	Security Security  `yaml:"security" json:"security"`
	// e2e checks after install, in the template's namespace.
	Smoke []Smoke `yaml:"smoke" json:"smoke"`

	Dir string `yaml:"-" json:"-"` // holds template.yaml and manifests.yaml
	// E2E is the template's e2e.yaml, which holds only test data.
	E2E E2EValues `yaml:"-" json:"-"`
}

// Metadata is a template's name, summary, version and docs.
type Metadata struct {
	catalog.Metadata `yaml:",inline"`
	// The upstream docs on how to use the application, not the repository root.
	Docs string `yaml:"docs" json:"docs"`
	// The application version: the tag of its main image. Objects get it as the label
	// app.kubernetes.io/version.
	Version string `yaml:"version" json:"version"`
}

// Security is what a template exposes and how its risks are mitigated.
type Security struct {
	Summary string                 `yaml:"summary" json:"summary"`
	Locales catalog.SummaryLocales `yaml:"locales" json:"locales,omitempty"`
	Risks   []Risk                 `yaml:"risks" json:"risks"`
	// Services with no sign-in or key of their own. Each needs an ingress NetworkPolicy; the risks
	// say why.
	Unauthenticated []string `yaml:"unauthenticated" json:"unauthenticated,omitempty"`
}

// Risk is a risk of a template and its mitigation.
type Risk struct {
	Risk       string      `yaml:"risk" json:"risk"`
	Mitigation string      `yaml:"mitigation" json:"mitigation"`
	Locales    RiskLocales `yaml:"locales" json:"locales,omitempty"`
}

// RiskLocales is the risk and mitigation in other languages, by BCP 47 tag. config/locales.yaml
// lists the tags each needs.
type RiskLocales map[catalog.Locale]struct {
	Risk       string `yaml:"risk" json:"risk"`
	Mitigation string `yaml:"mitigation" json:"mitigation"`
}

// Service is a workload in manifests.yaml.
type Service struct {
	Name string `yaml:"name" json:"name"`
	// The Pod Security Standard level its pods meet.
	PodSecurity string           `yaml:"podSecurity" json:"podSecurity"`
	Ports       []Port           `yaml:"ports" json:"ports,omitempty"`
	Resources   ServiceResources `yaml:"resources" json:"resources"`
	// Public hostnames; rendering then needs a domain and a ClusterIssuer.
	Domains []string `yaml:"domains" json:"domains,omitempty"`
}

// ServiceResources is the requests and limits of a service, by level and resource name.
type ServiceResources map[string]map[string]string

// Port is a named port of a service.
type Port struct {
	Name string `yaml:"name" json:"name"`
	Port int    `yaml:"port" json:"port"`
}

// Input is a value the person installing gives (required) or the installer generates into a
// Secret (generate).
type Input struct {
	// camelCase.
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	// A short name for the field, in English.
	Label    string            `yaml:"label" json:"label,omitempty"`
	Locales  InputLocales      `yaml:"locales" json:"locales,omitempty"`
	Generate *catalog.Generate `yaml:"generate" json:"generate,omitempty"`
	Required bool              `yaml:"required" json:"required,omitempty"`
	// The value when a deploy does not give the input.
	Default string `yaml:"default" json:"default,omitempty"`
	Secret  *struct {
		Name string `yaml:"name" json:"name"`
		Key  string `yaml:"key" json:"key"`
	} `yaml:"secret" json:"secret,omitempty"`
	Validation struct {
		// Regular expression the whole value matches.
		Pattern string `yaml:"pattern" json:"pattern"`
	} `yaml:"validation" json:"validation"`
}

// InputLocales is the label and description in other languages, by BCP 47 tag.
// config/locales.yaml lists the tags each needs.
type InputLocales map[catalog.Locale]struct {
	Label       string `yaml:"label" json:"label"`
	Description string `yaml:"description" json:"description"`
}

// Smoke is an e2e check after install.
type Smoke struct {
	Name     string `yaml:"name" json:"name"`
	Resource string `yaml:"resource" json:"resource"`
	// What the object reaches, as a kubectl wait --for value. A deployment, statefulset or daemonset
	// has none: it is ready when its rollout is complete.
	Condition string `yaml:"condition" json:"condition,omitempty"`
	Timeout   string `yaml:"timeout" json:"timeout"`
	// GET through a port-forward to the Service.
	HTTP *struct {
		Service  string `yaml:"service" json:"service"`
		Port     int    `yaml:"port" json:"port"`
		Path     string `yaml:"path" json:"path"`
		Status   int    `yaml:"status" json:"status"`
		Contains string `yaml:"contains" json:"contains,omitempty"`
	} `yaml:"http" json:"http,omitempty"`
	// One read-only SELECT on the CloudNativePG primary.
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
		if err := yaml.Unmarshal(data, t); err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if t.E2E, err = loadE2E(filepath.Join(t.Dir, "e2e.yaml")); err != nil {
			return nil, err
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
