package catalog

import (
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
	got := UnpinnedImages(manifest, []string{"ghcr.io/a/sidecar"})
	want := []string{
		"Deployment/op op: ghcr.io/a/op:1.0",
		"CronJob/clean clean: localhost:5000/clean:2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
