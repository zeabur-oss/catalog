package catalog

import (
	"reflect"
	"slices"
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

// A release pull request moves the change files into CHANGELOG.md before the
// tag exists, so the Breaks: lines of the sections above the previous
// release still count.
func TestUnreleasedChangelog(t *testing.T) {
	changelog := "# Changelog\n\n## 0.4.0 (2026-11-07)\n\nBreaks: builds/zeabur-builder StatefulSet zot/zot\n\n## 0.3.0 (2026-11-04)\n\nBreaks: old/one Deployment a/b\n"
	t.Run("the sections above the previous release count", func(t *testing.T) {
		got, err := UnreleasedChangelog(changelog, "0.3.0")
		if err != nil {
			t.Fatal(err)
		}
		breaks, err := ParseBreaks(got)
		if err != nil {
			t.Fatal(err)
		}
		want := []Break{{Component: "builds/zeabur-builder", Kind: "StatefulSet", Namespace: "zot", Name: "zot"}}
		if !slices.Equal(breaks, want) {
			t.Errorf("got %v, want %v", breaks, want)
		}
	})
	t.Run("once the release is tagged its section no longer counts", func(t *testing.T) {
		got, err := UnreleasedChangelog(changelog, "0.4.0")
		if err != nil {
			t.Fatal(err)
		}
		if breaks, _ := ParseBreaks(got); len(breaks) != 0 {
			t.Errorf("got %v, want none", breaks)
		}
	})
	t.Run("a previous release missing from the changelog is an error", func(t *testing.T) {
		if _, err := UnreleasedChangelog(changelog, "0.9.0"); err == nil {
			t.Error("want an error")
		}
	})
}
