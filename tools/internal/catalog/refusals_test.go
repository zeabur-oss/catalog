package catalog

import (
	"reflect"
	"testing"
)

// ParseRefusals reads kubectl's stderr from a --dry-run=server update. A line
// is a refusal only when every cause in it says the field can't change; any
// other error line is returned as unread, so the check reports it instead of
// passing.
func TestParseRefusals(t *testing.T) {
	cases := []struct {
		rule   string
		out    string
		want   []Refusal
		unread bool
	}{
		{
			rule: "a StatefulSet spec change other than the template is a refusal of spec",
			out:  `Error from server (Invalid): StatefulSet.apps "zot" is invalid: spec: Forbidden: updates to statefulset spec for fields other than 'replicas', 'ordinals', 'template', 'updateStrategy', 'revisionHistoryLimit', 'persistentVolumeClaimRetentionPolicy' and 'minReadySeconds' are forbidden`,
			want: []Refusal{{"zot", "zot", "StatefulSet", "zot", []string{"spec"}}},
		},
		{
			rule: "a field that is immutable is a refusal of that field",
			out:  `Error from server (Invalid): Job.batch "migrate" is invalid: spec.template: Invalid value: core.PodTemplateSpec{}: field is immutable`,
			want: []Refusal{{"zot", "zot", "Job", "migrate", []string{"spec.template"}}},
		},
		{
			rule: "several fields that can't change are one refusal naming each",
			out:  `Error from server (Invalid): Service "web" is invalid: [spec.clusterIPs[0]: Invalid value: []string{"10.96.0.9"}: may not change once set, spec.clusterIP: Invalid value: "10.96.0.9": field is immutable]`,
			want: []Refusal{{"zot", "zot", "Service", "web", []string{"spec.clusterIPs[0]", "spec.clusterIP"}}},
		},
		{
			rule:   "an immutable field next to another error is unread, not a refusal",
			out:    `Error from server (Invalid): Job.batch "migrate" is invalid: [spec.selector: Required value, spec.template: Invalid value: core.PodTemplateSpec{}: field is immutable]`,
			unread: true,
		},
		{
			rule:   "a value that is invalid but could change is unread",
			out:    `Error from server (Invalid): Deployment.apps "web" is invalid: spec.replicas: Invalid value: -1: must be greater than or equal to 0`,
			unread: true,
		},
		{
			rule:   "an error that isn't about a field is unread",
			out:    `Error from server (NotFound): namespaces "zot" not found`,
			unread: true,
		},
		{
			rule: "a warning is neither",
			out:  `Warning: unknown field "spec.foo"`,
		},
	}
	for _, c := range cases {
		t.Run(c.rule, func(t *testing.T) {
			got, unread := ParseRefusals(c.out, "zot", "zot")
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("refusals: got %v, want %v", got, c.want)
			}
			if (len(unread) > 0) != c.unread {
				t.Errorf("unread: got %q, want unread=%v", unread, c.unread)
			}
		})
	}
}
