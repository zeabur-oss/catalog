package catalog

import (
	"reflect"
	"slices"
	"strings"
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
	got := UnpinnedImages(manifest, []string{"ghcr.io/a/sidecar"})
	want := []string{
		"Deployment/op op: ghcr.io/a/op:1.0",
		"CronJob/clean clean: localhost:5000/clean:2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSplitImagesFindsEveryWorkloadKind(t *testing.T) {
	const digest = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	var b strings.Builder
	for _, k := range []string{"Deployment", "StatefulSet", "DaemonSet", "Job"} {
		b.WriteString("apiVersion: v1\nkind: " + k + "\nmetadata: {name: x}\nspec:\n  template:\n    spec:\n" +
			"      initContainers:\n        - {name: i, image: reg/" + k + "-init:1" + digest + "}\n" +
			"      containers:\n        - {name: c, image: reg/" + k + ":1" + digest + "}\n---\n")
	}
	b.WriteString(`apiVersion: batch/v1
kind: CronJob
metadata: {name: clean}
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - {name: c, image: reg/cron:1` + digest + `}
            - {name: dup, image: reg/cron:1` + digest + `}
---
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
    - {name: c, image: reg/pod:1` + digest + `}
    - {name: loose, image: reg/loose:1}
---
apiVersion: v1
kind: Pod
metadata:
  name: t
  annotations: {helm.sh/hook: test}
spec:
  containers:
    - {name: c, image: reg/test:1` + digest + `}
`)
	pinned, unpinned := SplitImages([]byte(b.String()))
	for _, name := range []string{"Deployment", "StatefulSet", "DaemonSet", "Job"} {
		for _, suffix := range []string{"", "-init"} {
			if !slices.Contains(pinned, "reg/"+name+suffix+":1"+digest) {
				t.Errorf("missing %s%s in %q", name, suffix, pinned)
			}
		}
	}
	for _, want := range []string{"reg/cron:1" + digest, "reg/pod:1" + digest} {
		if n := count(pinned, want); n != 1 {
			t.Errorf("%s listed %d times, want once", want, n)
		}
	}
	if slices.Contains(pinned, "reg/test:1"+digest) {
		t.Error("a Helm test hook image was listed")
	}
	if !slices.IsSorted(pinned) {
		t.Errorf("not sorted: %q", pinned)
	}
	if !reflect.DeepEqual(unpinned, []string{"reg/loose:1"}) {
		t.Errorf("unpinned = %q", unpinned)
	}
}

func count(l []string, s string) int {
	n := 0
	for _, v := range l {
		if v == s {
			n++
		}
	}
	return n
}
