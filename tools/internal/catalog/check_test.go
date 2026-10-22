package catalog

import (
	"reflect"
	"testing"
)

func TestLicenseProblems(t *testing.T) {
	l := Licenses{OSIApproved: []string{"Apache-2.0", "MIT"}, OptInOnly: []string{"SSPL-1.0"}}
	optional := map[string]string{"single-node": "optional", "multi-node": "optional"}
	tests := []struct {
		license  string
		defaults map[string]string
		want     []string
	}{
		{"Apache-2.0 OR MIT", map[string]string{"single-node": "recommended"}, nil},
		{"Apache-2.0 AND SSPL-1.0", optional, nil},
		{"Apache-2.0 AND SSPL-1.0", map[string]string{"single-node": "optional", "multi-node": "recommended"},
			[]string{"license SSPL-1.0 is opt-in only, so multi-node cannot be recommended"}},
		{"BUSL-1.1", optional, []string{"license BUSL-1.1 is not OSI-approved"}},
	}
	for _, tt := range tests {
		got := l.problems(&Component{License: tt.license, Defaults: tt.defaults})
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s %v: got %q, want %q", tt.license, tt.defaults, got, tt.want)
		}
	}
	if got := l.NotOSIApproved("(MIT AND SSPL-1.0) OR SSPL-1.0"); !reflect.DeepEqual(got, []string{"SSPL-1.0"}) {
		t.Errorf("NotOSIApproved: got %q", got)
	}
}
