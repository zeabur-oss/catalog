package main

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// manifestDoc is one object of a rendered release.
type manifestDoc struct {
	Kind, Name string
	// Namespace is the object's own, or the release's when it names none.
	Namespace string
	hook      bool // a Helm hook
	text      string
}

// manifestDocs splits a rendered release into its objects.
func manifestDocs(data []byte, defaultNamespace string) []manifestDoc {
	var out []manifestDoc
	for _, doc := range strings.Split(string(data), "\n---") {
		var o struct {
			Kind     string
			Metadata struct {
				Name, Namespace string
				Annotations     map[string]string
			}
		}
		if yaml.Unmarshal([]byte(doc), &o) != nil || o.Kind == "" {
			continue
		}
		_, hook := o.Metadata.Annotations["helm.sh/hook"]
		out = append(out, manifestDoc{o.Kind, o.Metadata.Name, cmp.Or(o.Metadata.Namespace, defaultNamespace), hook, doc})
	}
	return out
}

// nsDocs is manifest documents to apply in one namespace.
type nsDocs struct {
	namespace, docs string
}

// byNamespace joins docs per namespace, because kubectl -n refuses objects
// that name another namespace.
func byNamespace(docs []manifestDoc) []nsDocs {
	joined := map[string]string{}
	for _, d := range docs {
		joined[d.Namespace] += d.text + "\n---\n"
	}
	var out []nsDocs
	for _, ns := range slices.Sorted(maps.Keys(joined)) {
		out = append(out, nsDocs{ns, joined[ns]})
	}
	return out
}
