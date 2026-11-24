package catalog

import "strings"

// IsReadme reports whether name, a file in a component or template directory, is one of its
// READMEs. They are docs: they change no install and no measurement.
func IsReadme(name string) bool {
	return strings.HasPrefix(name, "README") && strings.HasSuffix(name, ".md")
}
