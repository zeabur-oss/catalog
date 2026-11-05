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

// refusalsManifest is release db of component x/y: a StatefulSet whose claim
// size is SIZE, which Kubernetes can't change in place; a Deployment whose
// image is IMAGE, which it can; and a Helm hook Job with image IMAGE, whose
// pod template can't change but which Helm creates again on every upgrade.
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
---
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate
  annotations: {helm.sh/hook: pre-upgrade}
spec:
  template:
    spec:
      restartPolicy: Never
      containers: [{name: migrate, image: "postgres:IMAGE"}]
`

// writeRender writes a render directory as catalog render does,
// <dir>/x/y/db.yaml, and returns dir.
func writeRender(t *testing.T, image, size string) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "x/y/db.yaml")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := strings.NewReplacer("IMAGE", image, "SIZE", size).Replace(refusalsManifest)
	if err := os.WriteFile(f, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The check tier applies one render and sends another over it with
// --dry-run=server; it fails on what the API server refuses to update and
// names the object (docs/contribute/tests.mdx, check tier).
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
	installed := writeRender(t, "17", "1Gi")
	is := func(kind string) func(catalog.Refusal) bool {
		return func(r catalog.Refusal) bool { return r.Kind == kind && r.Release == "db" && r.Namespace == "db" }
	}

	t.Run("a StatefulSet whose volumeClaimTemplates change is refused", func(t *testing.T) {
		found, err := refused(c, installed, writeRender(t, "18", "2Gi"))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(found, is("StatefulSet")) {
			t.Errorf("got %v, want StatefulSet db/db", found)
		}
		// The Deployment's image changed in the same render; that updates in place.
		if slices.ContainsFunc(found, is("Deployment")) {
			t.Errorf("got %v, want no Deployment", found)
		}
	})

	// The hook Job's template changes here too; hooks are left out.
	t.Run("a render that changes only what updates in place or is a Helm hook passes", func(t *testing.T) {
		found, err := refused(c, installed, writeRender(t, "18", "1Gi"))
		if err != nil {
			t.Fatal(err)
		}
		if found != nil {
			t.Errorf("got %v, want nothing", found)
		}
	})

	t.Run("a release that renders the same is skipped", func(t *testing.T) {
		found, err := refused(c, installed, installed)
		if err != nil {
			t.Fatal(err)
		}
		if found != nil {
			t.Errorf("got %v, want nothing", found)
		}
	})
}

// A change file may declare an object it intends to break with a
// `Breaks: <component> <Kind> <namespace>/<name>` line. An upgrade refusal
// that matches one passes; every other refusal fails, and so does a line that
// matches no refusal.
func TestDeclaredBreaks(t *testing.T) {
	zot := catalog.Refusal{Release: "zot", Namespace: "zot", Kind: "StatefulSet", Name: "zot", Fields: []string{"spec"}}
	upgrade := refusal{component: "builds/zeabur-builder", when: "upgrading multi-node from 0.3.0", upgrade: true, Refusal: zot}
	switched := refusal{component: "builds/zeabur-builder", when: "switching single-node to multi-node", Refusal: zot}
	declared := []catalog.Break{{Component: "builds/zeabur-builder", Kind: "StatefulSet", Namespace: "zot", Name: "zot"}}

	t.Run("a declared upgrade refusal passes", func(t *testing.T) {
		errs := judgeRefusals([]refusal{upgrade}, declared)
		if len(errs) != 0 {
			t.Errorf("got %v, want none", errs)
		}
	})
	t.Run("an undeclared refusal fails", func(t *testing.T) {
		errs := judgeRefusals([]refusal{upgrade}, nil)
		if len(errs) != 1 {
			t.Errorf("got %v, want one error", errs)
		}
	})
	t.Run("a declaration does not cover a profile switch", func(t *testing.T) {
		errs := judgeRefusals([]refusal{switched, upgrade}, declared)
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "switching") {
			t.Errorf("got %v, want the switch to fail", errs)
		}
	})
	t.Run("a declaration that matches nothing is an error", func(t *testing.T) {
		errs := judgeRefusals(nil, declared)
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "zot/zot") {
			t.Errorf("got %v, want the stale declaration named", errs)
		}
	})
}

// Only Breaks: lines count; the rest of a change file is left alone.
func TestParseBreaks(t *testing.T) {
	text := "---\ndefault: minor\n---\n\n# Title\n\nBreaks: builds/zeabur-builder StatefulSet zot/zot\nBreaks: a/b Deployment x/y\nSome Breaks: text in a sentence.\n"
	got, err := catalog.ParseBreaks(text)
	if err != nil {
		t.Fatal(err)
	}
	want := []catalog.Break{
		{Component: "builds/zeabur-builder", Kind: "StatefulSet", Namespace: "zot", Name: "zot"},
		{Component: "a/b", Kind: "Deployment", Namespace: "x", Name: "y"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if _, err := catalog.ParseBreaks("Breaks: only-two fields\n"); err == nil {
		t.Error("a malformed Breaks: line should be an error")
	}
}
