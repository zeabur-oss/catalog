package catalog

import (
	"reflect"
	"testing"
)

func TestParseRefusals(t *testing.T) {
	cases := []struct {
		name, out string
		want      []Refusal
		unread    bool
	}{
		{
			name: "statefulset spec",
			out:  `Error from server (Invalid): StatefulSet.apps "zot" is invalid: spec: Forbidden: updates to statefulset spec for fields other than 'replicas', 'ordinals', 'template', 'updateStrategy', 'revisionHistoryLimit', 'persistentVolumeClaimRetentionPolicy' and 'minReadySeconds' are forbidden`,
			want: []Refusal{{"zot", "zot", "StatefulSet", "zot", []string{"spec"}}},
		},
		{
			name:   "a required value among them",
			out:    `Error from server (Invalid): Job.batch "migrate" is invalid: [spec.selector: Required value, spec.template: Invalid value: core.PodTemplateSpec{}: field is immutable]`,
			unread: true,
		},
		{
			name: "job template",
			out:  `Error from server (Invalid): Job.batch "migrate" is invalid: spec.template: Invalid value: core.PodTemplateSpec{}: field is immutable`,
			want: []Refusal{{"zot", "zot", "Job", "migrate", []string{"spec.template"}}},
		},
		{
			name: "service clusterIP",
			out:  `Error from server (Invalid): Service "web" is invalid: [spec.clusterIPs[0]: Invalid value: []string{"10.96.0.9"}: may not change once set, spec.clusterIP: Invalid value: "10.96.0.9": field is immutable]`,
			want: []Refusal{{"zot", "zot", "Service", "web", []string{"spec.clusterIPs[0]", "spec.clusterIP"}}},
		},
		{
			name:   "a value that is wrong, not fixed",
			out:    `Error from server (Invalid): Deployment.apps "web" is invalid: spec.replicas: Invalid value: -1: must be greater than or equal to 0`,
			unread: true,
		},
		{
			name:   "another error",
			out:    `Error from server (NotFound): namespaces "zot" not found`,
			unread: true,
		},
		{
			name: "a warning",
			out:  `Warning: unknown field "spec.foo"`,
		},
	}
	for _, c := range cases {
		got, unread := ParseRefusals(c.out, "zot", "zot")
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
		if (len(unread) > 0) != c.unread {
			t.Errorf("%s: unread %q", c.name, unread)
		}
	}
}
