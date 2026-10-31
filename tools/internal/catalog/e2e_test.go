package catalog

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResidueVolumeFollowsClaim(t *testing.T) {
	added := []string{"persistentvolumes /pvc-1", "persistentvolumes /pvc-2", "persistentvolumes /pvc-3"}
	claims := map[string]string{
		"persistentvolumes /pvc-1": "persistentvolumeclaims zot/zot-pvc-zot-0",
		"persistentvolumes /pvc-2": "persistentvolumeclaims other/data",
	}
	keepsClaims := []*Component{{Leftovers: []string{"persistentvolumeclaims"}}}
	if got, want := residue(keepsClaims, added, claims), []string{"persistentvolumes /pvc-3"}; !slices.Equal(got, want) {
		t.Errorf("claims kept: residue = %q, want %q", got, want)
	}
	keepsNothing := []*Component{{}}
	if got := residue(keepsNothing, added, claims); !slices.Equal(got, added) {
		t.Errorf("nothing kept: residue = %q, want %q", got, added)
	}
}

func TestSharedDefaultAccountSurvivesOneComponentsUndo(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$@\" >> " + log + "\ncat >> " + log + "\n"
	if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	cat := &Catalog{}
	tester := &Tester{Catalog: cat}
	var comps []*Component
	for _, id := range []string{"autoscaling/keda", "autoscaling/keda-http"} {
		c := &Component{ID: id, Releases: []Release{{Name: "app-" + path.Base(id), Namespace: "keda", Manifests: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"}}}
		c.addDefaultAccounts()
		comps = append(comps, c)
	}
	r := newRun()
	for _, c := range comps {
		if err := tester.install(r, c); err != nil {
			t.Fatal(err)
		}
	}
	os.Remove(log)
	for _, u := range r.undo {
		u()
	}
	data, _ := os.ReadFile(log)
	if strings.Contains(string(data), "ServiceAccount") {
		t.Errorf("a component's undo touched the shared account: %s", data)
	}
	for _, c := range comps {
		if r.releases[c.ID+"/default-account-keda"] {
			t.Errorf("%s registered the shared account for removal", c.ID)
		}
	}
}

func TestSmokeFailureShowsWhyTheWaitedObjectIsNotReady(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$*" in
wait*) echo "timed out waiting for the condition" >&2; exit 1 ;;
*describe*) echo "Warning  FailedMount  pvc smoke-storage not bound" ;;
*"get pods"*) echo "smoke-0  0/1  Pending" ;;
*"get events"*) echo "Warning  FailedScheduling  0/1 nodes are available" ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	tester := &Tester{}
	c := &Component{ID: "databases/x", Smoke: []Smoke{{
		Name: "standalone",
		Wait: []string{"-n catalog-smoke mariadb/smoke --for=condition=Ready"},
	}}}
	err := tester.smoke(c)
	if err == nil {
		t.Fatal("smoke passed although the wait timed out")
	}
	for _, want := range []string{"FailedMount", "smoke-0  0/1  Pending", "FailedScheduling"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not show %q: %v", want, err)
		}
	}
}
