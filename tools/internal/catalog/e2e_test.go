package catalog

import (
	"slices"
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
