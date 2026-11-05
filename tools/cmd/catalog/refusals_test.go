package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

const refusalsManifest = `apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db}
spec:
  serviceName: db
  selector: {matchLabels: {app: db}}
  template:
    metadata: {labels: {app: db}}
    spec:
      containers: [{name: db, image: "postgres:IMAGE"}]
  volumeClaimTemplates:
    - metadata: {name: data}
      spec:
        accessModes: [ReadWriteOnce]
        resources: {requests: {storage: SIZE}}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers: [{name: web, image: "nginx:IMAGE"}]
`

// writeRender writes the release db of component x/y as catalog render would.
func writeRender(t *testing.T, image, size string) string {
	dir := t.TempDir()
	f := filepath.Join(dir, "x/y/db.yaml")
	os.MkdirAll(filepath.Dir(f), 0o755)
	manifest := strings.NewReplacer("IMAGE", image, "SIZE", size).Replace(refusalsManifest)
	if err := os.WriteFile(f, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRefused(t *testing.T) {
	if _, err := exec.LookPath("kube-apiserver"); err != nil {
		t.Skip("needs the dev shell's kube-apiserver and etcd")
	}
	api, err := startAPIServer()
	if err != nil {
		t.Fatal(err)
	}
	defer api.stop()
	c := &catalog.Component{ID: "x/y", Releases: []catalog.Release{{Name: "db", Namespace: "db"}}}
	from := writeRender(t, "17", "1Gi")

	found, err := refused(c, from, writeRender(t, "18", "2Gi"))
	if err != nil {
		t.Fatal(err)
	}
	is := func(kind string) func(catalog.Refusal) bool {
		return func(r catalog.Refusal) bool { return r.Kind == kind && r.Release == "db" && r.Namespace == "db" }
	}
	if !slices.ContainsFunc(found, is("StatefulSet")) {
		t.Errorf("a new volume size: got %v, want the StatefulSet", found)
	}
	if slices.ContainsFunc(found, is("Deployment")) {
		t.Errorf("a new image: got %v, the Deployment updates in place", found)
	}

	found, err = refused(c, from, writeRender(t, "18", "1Gi"))
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Errorf("a new image only: got %v, want nothing", found)
	}
}
