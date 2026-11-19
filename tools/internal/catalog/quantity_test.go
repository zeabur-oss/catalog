package catalog

import (
	"regexp"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestQuantityPatternAgreesWithKubernetes(t *testing.T) {
	pattern := regexp.MustCompile(quantityPattern)
	for _, q := range []string{"1", "100m", "256Mi", "1.5Gi", "2e3", "1E6", ".5", "5.", "0", "1k", "1Ki", "10n",
		"", "abc", "1 Gi", "1Gx", "1mi", "--1", "1ee3"} {
		_, err := resource.ParseQuantity(q)
		if pattern.MatchString(q) != (err == nil) {
			t.Errorf("%q: pattern says %v, ParseQuantity says %v", q, pattern.MatchString(q), err)
		}
	}
}
