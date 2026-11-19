package catalog

import (
	"bytes"
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
)

// E2EConfig is config/e2e.yaml: what e2e runs need that the catalog does not
// declare.
type E2EConfig struct {
	// Overrides are values merged after the profile values, per component and release.
	Overrides map[string]map[string]string `yaml:"overrides"`
	// Replacements are test-only text replacements in manifests, per component and release.
	Replacements map[string]map[string]map[string]string `yaml:"replacements"`
	// Unready lists components whose workloads cannot become ready without
	// credentials that CI does not have.
	Unready []string `yaml:"unready"`
	// SmokeRepresentative is what the smoke tier tests when a shared file changes.
	SmokeRepresentative []string `yaml:"smokeRepresentative"`
	// SkipSmoke maps a component and a smoke check to the reason e2e skips it.
	SkipSmoke map[string]map[string]string `yaml:"skipSmoke"`
	// NodeSetup maps a component to a shell script that runs in every node
	// before the component installs.
	NodeSetup map[string]string `yaml:"nodeSetup"`
	// TemplateInputs are input values for every template that declares the input.
	TemplateInputs map[string]string `yaml:"templateInputs"`
}

// LoadE2EConfig reads the file at path. It fails on a key it does not know.
func LoadE2EConfig(path string) (E2EConfig, error) {
	var c E2EConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return E2EConfig{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
