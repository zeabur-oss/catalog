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

// An upgrade refusal passes only when the pull request adds a change file
// whose frontmatter says `default: major`. A profile switch never passes.
func TestJudgeRefusals(t *testing.T) {
	zot := catalog.Refusal{Release: "zot", Namespace: "zot", Kind: "StatefulSet", Name: "zot", Fields: []string{"spec"}}
	upgrade := refusal{component: "builds/zeabur-builder", when: "multi-node", upgrade: true, Refusal: zot}
	switched := refusal{component: "builds/zeabur-builder", when: "single-node to multi-node", Refusal: zot}

	t.Run("an upgrade refusal without a major change file fails", func(t *testing.T) {
		errs := judgeRefusals([]refusal{upgrade}, false)
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "StatefulSet zot/zot") {
			t.Errorf("got %v, want the object named", errs)
		}
	})
	t.Run("the same refusal with a major change file passes", func(t *testing.T) {
		if errs := judgeRefusals([]refusal{upgrade}, true); len(errs) != 0 {
			t.Errorf("got %v, want none", errs)
		}
	})
	t.Run("a profile switch refusal fails even with a major change file", func(t *testing.T) {
		errs := judgeRefusals([]refusal{switched}, true)
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "switch") {
			t.Errorf("got %v, want the switch to fail", errs)
		}
	})
}

// gitRepo returns a repository with one commit and its hash.
func gitRepo(t *testing.T) (dir, base string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	os.MkdirAll(filepath.Join(dir, ".changeset"), 0o755)
	os.WriteFile(filepath.Join(dir, ".changeset/old.md"), []byte("---\ndefault: major\n---\n\n# Old\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "base")
	return dir, git("rev-parse", "HEAD")
}

// addChange commits a change file with the given level.
func addChange(t *testing.T, dir, name, level string) {
	t.Helper()
	text := "---\ndefault: " + level + "\n---\n\n# Change\n\nSteps with default: major in the text.\n"
	if err := os.WriteFile(filepath.Join(dir, ".changeset", name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", name}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func TestAddsMajorChangeFile(t *testing.T) {
	t.Run("a major change file the pull request adds counts", func(t *testing.T) {
		dir, base := gitRepo(t)
		addChange(t, dir, "new.md", "major")
		if got, err := addsMajorChangeFile(dir, base); err != nil || !got {
			t.Errorf("got %v, %v, want true", got, err)
		}
	})
	t.Run("a minor or patch change file does not", func(t *testing.T) {
		dir, base := gitRepo(t)
		addChange(t, dir, "a.md", "minor")
		addChange(t, dir, "b.md", "patch")
		if got, err := addsMajorChangeFile(dir, base); err != nil || got {
			t.Errorf("got %v, %v, want false", got, err)
		}
	})
	t.Run("a major change file already in the base does not", func(t *testing.T) {
		dir, base := gitRepo(t)
		if got, err := addsMajorChangeFile(dir, base); err != nil || got {
			t.Errorf("got %v, %v, want false", got, err)
		}
	})
}
