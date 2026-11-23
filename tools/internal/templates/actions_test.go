package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestInputDefaultFillsAnInputThatIsNotGiven(t *testing.T) {
	dir := t.TempDir()
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: app}\ndata: {url: '{{ .Inputs.url }}'}\n"
	if err := os.WriteFile(filepath.Join(dir, "manifests.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	r := &Template{}
	if err := yaml.Unmarshal([]byte(`{"inputs":[{"name":"url","description":"u","default":"https://a.test","validation":{"pattern":"^https://.*$"}}]}`), r); err != nil {
		t.Fatal(err)
	}
	r.Dir = dir
	for given, want := range map[string]string{"": "https://a.test", "https://b.test": "https://b.test"} {
		opts := RenderOptions{Namespace: "test"}
		if given != "" {
			opts.Inputs = map[string]string{"url": given}
		}
		out, err := Render(r, opts)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), "url: '"+want+"'") {
			t.Errorf("given %q: got %s", given, out)
		}
	}
	r.Inputs[0].Default = "http://a.test"
	if _, err := Render(r, RenderOptions{Namespace: "test"}); err == nil {
		t.Error("a default that does not match its pattern was accepted")
	}
}
