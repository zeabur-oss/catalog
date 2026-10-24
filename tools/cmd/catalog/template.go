package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// Template e2e runs on the smaller profile; templates fit on one node.
const templateProfile = "single-node"

func template(fs *flag.FlagSet, r *repo) error {
	if len(os.Args) < 3 {
		return errors.New("usage: catalog template render|inputs|lint|e2e [flags] <id>")
	}
	action := os.Args[2]
	namespace := fs.String("namespace", "default", "namespace to render for")
	domain := fs.String("domain", "", "hostname for services with domains")
	issuer := fs.String("cluster-issuer", "", "cert-manager ClusterIssuer for services with domains")
	values := fs.String("values", "", "JSON object of input values")
	setup := fs.String("setup", "", "objects to apply in the namespace first (e2e)")
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
			return templates.Lint(r.cat, t, manifest, *namespace, templateProfile, cfg.Versions)
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
	case "e2e":
		tester, err := newTester(r.cat, templateProfile)
		if err != nil {
			return err
		}
		values, err := e2eValues(r.templates, t)
		if err != nil {
			return err
		}
		maps.Copy(values.Inputs, inputs)
		if *setup != "" {
			objects, err := os.ReadFile(*setup)
			if err != nil {
				return err
			}
			values.Setup += "\n---\n" + string(objects)
		}
		return templates.E2E(r.cat, tester, t, values, cfg.Versions)
	}
	return fmt.Errorf("unknown template action %q", action)
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
