package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheKeyKeepsPartsApart(t *testing.T) {
	keys := map[string][]string{}
	for _, parts := range [][]string{{"a/b", "c"}, {"a_b", "c"}, {"a", "b/c"}, {"ab", "c"}, {"a", "bc"}} {
		k := cacheKey(parts...)
		if prev, ok := keys[k]; ok {
			t.Errorf("%q and %q share the cache key %s", prev, parts, k)
		}
		keys[k] = parts
	}
}

func TestPullCacheStaysInCacheDir(t *testing.T) {
	old := CacheDir
	CacheDir = t.TempDir()
	defer func() { CacheDir = old }()
	outside := filepath.Join(CacheDir, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	rel := Release{Chart: "c", Repo: "https://example.com", Version: "../../outside"}
	if _, err := pull(rel); err == nil {
		t.Fatal("pull without helm succeeded")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("pull wrote into %s: %v", outside, entries)
	}
}
