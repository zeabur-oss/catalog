package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// Template e2e runs on the smaller profile; templates fit on one node.
const templateProfile = "single-node"

func template(fs *flag.FlagSet, r *repo) error {
	if len(os.Args) < 3 {
		return errors.New("usage: catalog template render|inputs|lint [flags] <id>")
	}
	action := os.Args[2]
	namespace := fs.String("namespace", "default", "namespace to render for")
	domain := fs.String("domain", "", "hostname for services with domains")
	issuer := fs.String("cluster-issuer", "", "cert-manager ClusterIssuer for services with domains")
	values := fs.String("values", "", "JSON object of input values")
	fs.Parse(os.Args[3:])
	if fs.NArg() != 1 {
		return errors.New("give one template ID")
	}
	t, err := templates.Get(r.templates, fs.Arg(0))
	if err != nil {
		return err
	}
	inputs, err := templates.ReadValues(*values)
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	opts := templates.RenderOptions{Namespace: *namespace, Domain: *domain, ClusterIssuer: *issuer, Inputs: inputs}
	switch action {
	case "render", "lint":
		manifest, err := templates.Render(t, opts)
		if err != nil {
			return err
		}
		if action == "lint" {
			schemas, err := requiredSchemas(r.cat, t, cfg.Versions)
			if err != nil {
				return err
			}
			defer os.RemoveAll(schemas)
			return templates.Lint(t, manifest, *namespace, schemas, cfg.Versions)
		}
		_, err = os.Stdout.Write(manifest)
		return err
	case "inputs":
		out, err := templates.GenerateInputs(t, *namespace)
		if err != nil {
			return err
		}
		_, err = fmt.Println(string(out))
		return err
	}
	return fmt.Errorf("unknown template action %q", action)
}

// requiredSchemas renders the components that t requires and writes the
// schemas of their CRDs to a new directory.
func requiredSchemas(cat *catalog.Catalog, t *templates.Template, versions []string) (string, error) {
	dir, err := os.MkdirTemp("", "catalog-template-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	order, err := cat.InstallOrder(t.Requires, templateProfile)
	if err != nil {
		return "", err
	}
	// A template without requires renders no components. CRDSchemas needs the directory.
	if err := os.Mkdir(filepath.Join(dir, "components"), 0o755); err != nil {
		return "", err
	}
	for _, c := range order {
		for _, rel := range c.Releases {
			out, err := catalog.Template(c, rel, templateProfile, versions[len(versions)-1])
			if err != nil {
				return "", fmt.Errorf("%s/%s: %w", c.ID, rel.Name, err)
			}
			f := filepath.Join(dir, "components", c.ID, rel.Name+".yaml")
			if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(f, out, 0o644); err != nil {
				return "", err
			}
		}
	}
	schemas, err := os.MkdirTemp("", "catalog-schemas-")
	if err != nil {
		return "", err
	}
	if err := catalog.CRDSchemas(filepath.Join(dir, "components"), schemas); err != nil {
		os.RemoveAll(schemas)
		return "", err
	}
	return schemas, nil
}

// e2eValues reads the test values for t from config/e2e.yaml: the shared
// inputs that t declares, then its own values.
func e2eValues(all []*templates.Template, t *templates.Template) (templates.E2EValues, error) {
	var cfg struct {
		Inputs    map[string]string              `yaml:"templateInputs"`
		Templates map[string]templates.E2EValues `yaml:"templates"`
	}
	if err := catalog.ReadYAML("config/e2e.yaml", &cfg); err != nil {
		return templates.E2EValues{}, err
	}
	for id := range cfg.Templates {
		if _, err := templates.Get(all, id); err != nil {
			return templates.E2EValues{}, fmt.Errorf("config/e2e.yaml: %w", err)
		}
	}
	v := cfg.Templates[t.ID]
	inputs := map[string]string{}
	for _, in := range t.Inputs {
		if s, ok := cfg.Inputs[in.Name]; ok {
			inputs[in.Name] = s
		}
	}
	maps.Copy(inputs, v.Inputs)
	v.Inputs = inputs
	return v, nil
}
