package main

import (
	"maps"

	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// Template e2e runs on the smaller profile; templates fit on one node.
const templateProfile = "single-node"

// templateArch is the only architecture a template's full test runs on.
const templateArch = "amd64"

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
