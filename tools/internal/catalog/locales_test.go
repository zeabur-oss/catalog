package catalog

import (
	"slices"
	"testing"
)

func TestEveryLocalesMapNeedsTheRequiredLanguages(t *testing.T) {
	doc := map[string]any{
		"metadata": map[string]any{"locales": map[string]any{"zh-TW": "x", "zh-CN": "x", "ja": "x"}},
		"inputs": []any{
			map[string]any{"locales": map[string]any{"zh-TW": "x"}},
		},
	}
	got := MissingLocales(doc, []string{"zh-TW", "zh-CN"})
	if want := []string{"inputs.0.locales: no zh-CN"}; !slices.Equal(got, want) {
		t.Errorf("MissingLocales = %q, want %q", got, want)
	}
}
