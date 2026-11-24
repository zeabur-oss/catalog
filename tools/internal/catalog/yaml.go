package catalog

import (
	"os"

	"go.yaml.in/yaml/v4"
)

// ReadYAML decodes the YAML file at path into v.
func ReadYAML(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}
