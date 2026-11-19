package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUnpinnedImages(t *testing.T) {
	const digest = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	manifest := []byte(`apiVersion: apps/v1
kind: Deployment
metadata: {name: op}
spec:
  template:
    spec:
      initContainers:
        - {name: init, image: ghcr.io/a/init:1.0` + digest + `}
      containers:
        - {name: op, image: ghcr.io/a/op:1.0}
        - {name: sidecar, image: ghcr.io/a/sidecar:1.0}
---
apiVersion: batch/v1
kind: CronJob
metadata: {name: clean}
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - {name: clean, image: localhost:5000/clean:2}
---
apiVersion: v1
kind: Pod
metadata:
  name: test-connection
  annotations: {helm.sh/hook: test}
spec:
  containers:
    - {name: wget, image: busybox}
`)
	got, err := UnpinnedImages(manifest, []string{"ghcr.io/a/sidecar"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Deployment/op op: ghcr.io/a/op:1.0",
		"CronJob/clean clean: localhost:5000/clean:2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

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
