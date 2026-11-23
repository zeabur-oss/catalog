package catalog

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

// quantityPatterns adds every "pattern" of the schema v that names a binary suffix: the quantities.
func quantityPatterns(v any, found map[string]bool) {
	switch v := v.(type) {
	case map[string]any:
		for k, x := range v {
			if s, ok := x.(string); ok && k == "pattern" && strings.Contains(s, "[KMGTPE]i") {
				found[s] = true
			}
			quantityPatterns(x, found)
		}
	case []any:
		for _, x := range v {
			quantityPatterns(x, found)
		}
	}
}

func TestQuantityPatternAgreesWithKubernetes(t *testing.T) {
	data, err := os.ReadFile("../../../schema/v1/component-file.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	quantityPatterns(schema, found)
	if len(found) == 0 {
		t.Fatal("component-file.schema.json has no quantity pattern")
	}
	for p := range found {
		pattern := regexp.MustCompile(p)
		for _, q := range []string{"1", "100m", "256Mi", "1.5Gi", "2e3", "1E6", ".5", "5.", "0", "1k", "1Ki", "10n",
			"", "abc", "1 Gi", "1Gx", "1mi", "--1", "1ee3"} {
			_, err := resource.ParseQuantity(q)
			if pattern.MatchString(q) != (err == nil) {
				t.Errorf("%q: pattern says %v, ParseQuantity says %v", q, pattern.MatchString(q), err)
			}
		}
	}
}
