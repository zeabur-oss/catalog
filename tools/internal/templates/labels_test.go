package templates_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestEveryObjectOfAnInstallHasItsLabels(t *testing.T) {
	dir := t.TempDir()
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata: {name: app, labels: {app: app}}\n---\napiVersion: v1\nkind: Service\nmetadata: {name: app}\nspec: {ports: [{port: 80}]}\n"
	if err := os.WriteFile(filepath.Join(dir, "manifests.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	r := &templates.Template{}
	if err := yaml.Unmarshal([]byte(`{"metadata":{"version":"1.2.3"},"services":[{"name":"app"}],"inputs":[{"name":"password","description":"password","generate":"random","secret":{"name":"app-secrets","key":"password"},"validation":{"pattern":"^[A-Za-z0-9_-]{43}$"}}]}`), r); err != nil {
		t.Fatal(err)
	}
	r.ID, r.Dir = "app", dir
	want := map[string]string{templates.TemplateLabel: "app", templates.InstanceLabel: "app", templates.VersionLabel: "1.2.3"}
	check := func(name string, labels map[string]string) {
		t.Helper()
		for k, v := range want {
			if labels[k] != v {
				t.Errorf("%s: label %s is %q, want %q", name, k, labels[k], v)
			}
		}
	}

	out, err := templates.Render(r, templates.RenderOptions{Namespace: "test"})
	if err != nil {
		t.Fatal(err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(out))
	for {
		var obj struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
		}
		if err := dec.Decode(&obj); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		check(obj.Kind, obj.Metadata.Labels)
		if obj.Kind == "ConfigMap" && obj.Metadata.Labels["app"] != "app" {
			t.Error("ConfigMap lost its own label")
		}
	}

	secrets, err := templates.GenerateInputs(r, "test")
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(secrets, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) == 0 {
		t.Fatal("no generated Secrets")
	}
	for _, item := range list.Items {
		check("generated Secret", item.Metadata.Labels)
	}
}
