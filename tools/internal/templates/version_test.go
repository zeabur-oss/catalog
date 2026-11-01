package templates_test

import (
	"testing"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestVersionNamesThePinnedImageWhoseTagItIs(t *testing.T) {
	const digest = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	manifests := "        image: docker.io/n8nio/n8n:2.45.3" + digest + "\n        image: docker.io/n8nio/runners:2.45.3" + digest + "\n"
	for _, c := range []struct {
		name, template, manifests string
		ok                        bool
	}{
		{"named and pinned", "metadata:\n  # renovate: depName=docker.io/n8nio/n8n\n  version: \"2.45.3\"\n", manifests, true},
		{"no comment", "metadata:\n  version: \"2.45.3\"\n", manifests, false},
		{"comment not above the version", "metadata:\n  # renovate: depName=docker.io/n8nio/n8n\n  name: n8n\n  version: \"2.45.3\"\n", manifests, false},
		{"image at another tag", "metadata:\n  # renovate: depName=docker.io/n8nio/n8n\n  version: \"2.45.3\"\n", "image: docker.io/n8nio/n8n:2.45.1" + digest + "\nimage: docker.io/n8nio/runners:2.45.3" + digest + "\n", false},
		{"only a longer name has the tag", "metadata:\n  # renovate: depName=n8nio/n8n\n  version: \"2.45.3\"\n", manifests, false},
		{"tag without a digest", "metadata:\n  # renovate: depName=docker.io/n8nio/n8n\n  version: \"2.45.3\"\n", "image: docker.io/n8nio/n8n:2.45.3\n", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := templates.CheckVersion([]byte(c.template), []byte(c.manifests), "2.45.3")
			if (err == nil) != c.ok {
				t.Fatalf("CheckVersion = %v, want ok %v", err, c.ok)
			}
		})
	}
}
