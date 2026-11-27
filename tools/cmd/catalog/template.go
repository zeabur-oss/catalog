package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"

	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// Template e2e runs on the smaller profile; templates fit on one node.
const templateProfile = "single-node"

// templateArch is the only architecture a template's full test runs on.
const templateArch = "amd64"

func template(fs *flag.FlagSet, r *repo) error {
	if len(os.Args) < 3 {
		return errors.New("usage: catalog template render|inputs [flags] <id>")
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
	opts := templates.RenderOptions{Namespace: *namespace, Domain: *domain, ClusterIssuer: *issuer, Inputs: inputs}
	switch action {
	case "render":
		manifest, err := templates.Render(t, opts)
		if err != nil {
			return err
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

// e2eValues returns the test values for t: the shared inputs of
// config/e2e.yaml that t declares, then t's own values from its e2e.yaml.
func e2eValues(r *repo, t *templates.Template) (templates.E2EValues, error) {
	v := t.E2E
	inputs := map[string]string{}
	for _, in := range t.Inputs {
		if s, ok := r.e2e.TemplateInputs[in.Name]; ok {
			inputs[in.Name] = s
		}
	}
	maps.Copy(inputs, v.Inputs)
	v.Inputs = inputs
	return v, nil
}
