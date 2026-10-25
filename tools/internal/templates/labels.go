package templates

import (
	"bytes"
	"errors"
	"io"
	"maps"
	"regexp"
	"slices"

	"go.yaml.in/yaml/v3"
)

// Labels of every top-level object of an install, and of the Secrets that
// GenerateInputs makes. With them, `kubectl delete <kinds> -l
// app.kubernetes.io/instance=<id>` removes one install from its namespace.
const (
	InstanceLabel = "app.kubernetes.io/instance"
	VersionLabel  = "app.kubernetes.io/version"
)

// Labels returns the labels of the objects of r.
func Labels(r *Template) map[string]string {
	return map[string]string{TemplateLabel: r.ID, InstanceLabel: r.ID, VersionLabel: r.Metadata.Version}
}

// labelValue is a Kubernetes label value that is not empty.
var labelValue = regexp.MustCompile(`^[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?$`)

// stamp adds labels to the metadata of every object in manifest.
func stamp(manifest []byte, labels map[string]string) ([]byte, error) {
	dec := yaml.NewDecoder(bytes.NewReader(manifest))
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
			continue
		}
		meta := mapping(doc.Content[0], "metadata")
		for _, k := range slices.Sorted(maps.Keys(labels)) {
			set(mapping(meta, "labels"), k, labels[k])
		}
		if err := enc.Encode(&doc); err != nil {
			return nil, err
		}
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// mapping returns the mapping under key in m, and adds it if it is missing.
func mapping(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
	return v
}

func set(m *yaml.Node, key, value string) {
	v := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
}
