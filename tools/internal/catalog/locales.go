package catalog

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
)

// ReadLocales returns the languages that config/locales.yaml requires in
// every locales map.
func ReadLocales(root string) ([]string, error) {
	var l struct {
		Required []string `yaml:"required"`
	}
	return l.Required, ReadYAML(filepath.Join(root, "config", "locales.yaml"), &l)
}

// MissingLocales returns a problem for each locales map in doc that does not
// have a required language.
func MissingLocales(doc any, required []string) []string {
	var out []string
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch v := v.(type) {
		case map[string]any:
			if l, ok := v["locales"].(map[string]any); ok {
				for _, lang := range required {
					if _, ok := l[lang]; !ok {
						out = append(out, fmt.Sprintf("%slocales: no %s", path, lang))
					}
				}
			}
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				walk(path+k+".", v[k])
			}
		case []any:
			for i, e := range v {
				walk(path+strconv.Itoa(i)+".", e)
			}
		}
	}
	walk("", doc)
	return out
}
