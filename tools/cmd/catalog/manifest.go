package main

import (
	"cmp"
	"maps"
	"slices"

	"sigs.k8s.io/yaml"

	"github.com/zeabur-oss/catalog/tools/internal/kubeobj"
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
func manifestDocs(data []byte, defaultNamespace string) ([]manifestDoc, error) {
	objs, err := kubeobj.Decode(data)
	if err != nil {
		return nil, err
	}
	var out []manifestDoc
	for _, o := range objs {
		if o.GetKind() == "" {
			continue
		}
		text, err := yaml.Marshal(o.Object)
		if err != nil {
			return nil, err
		}
		_, hook := o.GetAnnotations()["helm.sh/hook"]
		out = append(out, manifestDoc{o.GetKind(), o.GetName(), cmp.Or(o.GetNamespace(), defaultNamespace), hook, string(text)})
	}
	return out, nil
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
