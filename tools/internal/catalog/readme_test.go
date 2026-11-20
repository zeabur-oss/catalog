package catalog

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestComponentReadmes(t *testing.T) {
	const ok = "## Introduction\n\nRoutes traffic.\n\n## Use it\n\nAdd a domain.\n\n## Remove it\n\nDomains stop.\n"
	setup := Input{Name: "bucket", Path: "bucket", Required: true}
	for _, c := range []struct {
		name   string
		inputs []Input
		en     string
		want   []string
	}{
		{"required sections", nil, ok, nil},
		{"input with a default needs no setup", []Input{{Name: "bucket", Path: "bucket", Required: true, Default: "b"}}, ok, nil},
		{"input from outside needs setup", []Input{setup}, ok, []string{"components/x.README.md: no ## Before you install"}},
		{"setup present", []Input{setup}, "## Introduction\n\n## Before you install\n\nMake a bucket.\n\n## Use it\n\n## Remove it\n", nil},
		{"order", nil, "## Introduction\n\n## Remove it\n\n## Use it\n", []string{`components/x.README.md: heading "## Use it"; the level-2 headings are Introduction, Before you install, Use it, Remove it, Troubleshooting, in that order`}},
		{"title", nil, "# X\n\n" + ok, []string{`components/x.README.md: level-1 heading "# X"; start with ## Introduction`}},
		{"heading in a code block", nil, ok + "\n```sh\n# a comment\n```\n", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, text string) {
				if err := os.MkdirAll(filepath.Join(root, "components"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "components", name), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write("x.README.md", c.en)
			comp := &Component{ID: "x", Inputs: c.inputs}
			if got := comp.readmeProblems(root, nil); !slices.Equal(got, c.want) {
				t.Errorf("got %q, want %q", got, c.want)
			}
			if got := comp.readmeProblems(root, []string{"zh-TW"}); !slices.Contains(got, "components/x.README.zh-TW.md is missing") {
				t.Errorf("a missing zh-TW README passed: %q", got)
			}
		})
	}
}
