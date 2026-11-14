package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestSchemasAreGeneratedFromTheGoTypes(t *testing.T) {
	t.Chdir("../../..")
	files, err := schemaFiles()
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		if got, err := os.ReadFile(filepath.Join("schema", "v1", name)); err != nil || !bytes.Equal(got, want) {
			t.Errorf("schema/v1/%s is not what the Go types give; run go run ./tools/cmd/catalog schema", name)
		}
	}
}

const webComponent = `apiVersion: catalog.zeabur.com/v1
kind: Component
metadata:
  name: Web
  summary: Serves a page.
  locales: {}
  license: Apache-2.0
  homepage: https://example.com
defaults: {single-node: recommended, multi-node: unsupported}
distributions: [k3s]
kubeVersion: ">=1.30.0-0"
releases:
  - name: web
    type: inline
    namespace: web
    manifests: |
      apiVersion: apps/v1
      kind: Deployment
      metadata: {name: web}
      spec:
        replicas: 2
        template:
          spec:
            containers:
              - name: web
                resources: {requests: {memory: 64Mi, cpu: 10m}, limits: {memory: 128Mi}}
      ---
      apiVersion: apps/v1
      kind: StatefulSet
      metadata: {name: db}
      spec:
        template:
          spec:
            containers: [{name: db}]
        volumeClaimTemplates:
          - metadata: {name: data}
            spec: {resources: {requests: {storage: 1Gi}}}
      ---
      apiVersion: apps/v1
      kind: DaemonSet
      metadata: {name: agent, namespace: kube-system}
      spec:
        template:
          spec:
            containers: [{name: agent}]
      ---
      apiVersion: v1
      kind: PersistentVolumeClaim
      metadata: {name: cache}
      spec: {resources: {requests: {storage: 5Gi}}}
detection: {crds: [webs.example.com]}
security: {summary: Reads nothing., podSecurity: restricted}
`

const blogTemplate = `apiVersion: catalog.zeabur.com/v1
kind: Template
metadata:
  name: Blog
  version: "1.0.0"
  summary: A blog.
  locales: {}
  license: MIT
  homepage: https://example.com
services:
  - name: blog
    podSecurity: restricted
    ports: [{name: http, port: 8080}]
    resources: {requests: {cpu: 10m, memory: 64Mi}, limits: {cpu: 100m, memory: 128Mi}}
security: {summary: Reads nothing., risks: []}
smoke:
  - {name: ready, resource: deployment/blog, condition: condition=Available, timeout: 60s}
`

// blogManifests has a Go template field, so it is not YAML until it is expanded.
const blogManifests = `# The blog.
apiVersion: v1
kind: ConfigMap
metadata:
  name: blog
  namespace: '{{ .Namespace }}'
`

// writeTestAPI writes the API of a catalog with components/web.yaml and one measurement of it
// to out/v1 in a new working directory.
func writeTestAPI(t *testing.T) {
	t.Helper()
	schemas, err := filepath.Abs("../../../schema/v1")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := os.CopyFS("schema/v1", os.DirFS(schemas)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("components", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("components/web.yaml", []byte(webComponent), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "measurements/components/web.json", measurement{Component: "web", Environments: []Environment{k3sEnv("amd64", 30, "2026-11-01")}})
	if err := os.MkdirAll("templates/blog", 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"template.yaml": blogTemplate, "manifests.yaml": blogManifests} {
		if err := os.WriteFile("templates/blog/"+name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cat, err := catalog.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := templates.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config{Versions: []string{"1.36.0", "1.37.0"}}
	cfg.K3s.Version = "v1.36.5+k3s1"
	if err := writeAPI(cat, ts, cfg, "out/v1", "0.5.0", strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
}

func TestAPIPublishesTheDeclarationRenderAndMeasurements(t *testing.T) {
	writeTestAPI(t)
	if err := checkAPI("out/v1"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("out/v1/components/web.json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Declaration  any
		Render       Render
		Measurements []Environment
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	var written any
	if err := catalog.ReadYAML("components/web.yaml", &written); err != nil {
		t.Fatal(err)
	}
	if err := remarshal(written, &written); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Declaration, written) {
		t.Errorf("declaration = %v, want components/web.yaml as written", got.Declaration)
	}

	if _, ok := got.Render.Profiles["multi-node"]; ok {
		t.Error("render has multi-node, where the component is unsupported")
	}
	workloads := map[string]RenderedWorkload{}
	for _, w := range got.Render.Profiles["single-node"].Workloads {
		workloads[w.Kind+"/"+w.Name] = w
	}
	quantity := func(l map[string]resource.Quantity, name string) string {
		q := l[name]
		return q.String()
	}
	if w := workloads["Deployment/web"]; w.Replicas == nil || *w.Replicas != 2 || w.Namespace != "web" || len(w.Containers) == 0 ||
		quantity(w.Containers[0].Requests, "memory") != "64Mi" || quantity(w.Containers[0].Limits, "memory") != "128Mi" {
		t.Errorf("Deployment web = %+v, want 2 replicas in web with its requests and limits", w)
	}
	if w := workloads["StatefulSet/db"]; w.Replicas == nil || *w.Replicas != 1 || len(w.VolumeClaimTemplates) == 0 ||
		w.VolumeClaimTemplates[0].Storage.String() != "1Gi" {
		t.Errorf("StatefulSet db = %+v, want 1 replica and a 1Gi claim template", w)
	}
	if w := workloads["DaemonSet/agent"]; w.Replicas != nil || w.Namespace != "kube-system" {
		t.Errorf("DaemonSet agent = %+v, want no replicas, in kube-system", w)
	}
	claims := got.Render.Profiles["single-node"].PersistentVolumeClaims
	if !slices.ContainsFunc(claims, func(c RenderedClaim) bool { return c.Name == "cache" && c.Storage.String() == "5Gi" }) {
		t.Errorf("persistentVolumeClaims = %+v, want cache with 5Gi", claims)
	}
	if len(got.Measurements) == 0 || got.Measurements[0].Pods[0].Memory.String() != "30Mi" {
		t.Errorf("measurements = %+v, want the run of measurements/components/web.json", got.Measurements)
	}
}

func TestAPIPublishesTheTemplateAsWritten(t *testing.T) {
	writeTestAPI(t)
	if err := checkAPI("out/v1"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("out/v1/templates/blog.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		ID           string
		SourceDigest string
		Template     any
		Manifests    string
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}

	var written any
	if err := catalog.ReadYAML("templates/blog/template.yaml", &written); err != nil {
		t.Fatal(err)
	}
	if err := remarshal(written, &written); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(file.Template, written) {
		t.Errorf("template = %v, want templates/blog/template.yaml as written", file.Template)
	}
	if file.Manifests != blogManifests {
		t.Errorf("manifests = %q, want templates/blog/manifests.yaml as written", file.Manifests)
	}
	digest, err := sourceDigest("templates/blog")
	if err != nil {
		t.Fatal(err)
	}
	if file.ID != "blog" || file.SourceDigest != digest {
		t.Errorf("id, sourceDigest = %q, %q, want blog, %q", file.ID, file.SourceDigest, digest)
	}

	var index CatalogFile
	if data, err = os.ReadFile("out/v1/catalog.json"); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	if want := []TemplateEntry{{ID: "blog", SourceDigest: digest}}; !reflect.DeepEqual(index.Templates, want) {
		t.Errorf("catalog.json templates = %v, want %v", index.Templates, want)
	}
	if _, err := os.Stat("out/v1/schema/template.schema.json"); err != nil {
		t.Error("the API does not publish template.schema.json")
	}
}

func TestAPICheckRejectsABrokenFile(t *testing.T) {
	for _, c := range []struct {
		name, file string
		breakIt    func(doc map[string]any)
	}{
		{"component without sourceDigest", "components/web.json", func(doc map[string]any) { delete(doc, "sourceDigest") }},
		{"template without manifests", "templates/blog.json", func(doc map[string]any) { delete(doc, "manifests") }},
		{"template with an unknown field", "templates/blog.json", func(doc map[string]any) {
			doc["template"].(map[string]any)["replicas"] = 2
		}},
		{"declaration with an unknown field", "components/web.json", func(doc map[string]any) {
			doc["declaration"].(map[string]any)["replicas"] = 2
		}},
		{"commit that is not a hash", "catalog.json", func(doc map[string]any) {
			doc["catalog"].(map[string]any)["commit"] = "main"
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			writeTestAPI(t)
			path := filepath.Join("out/v1", c.file)
			var doc map[string]any
			data, err := os.ReadFile(path)
			if err == nil {
				err = json.Unmarshal(data, &doc)
			}
			if err != nil {
				t.Fatal(err)
			}
			c.breakIt(doc)
			writeFile(t, path, doc)
			if err := checkAPI("out/v1"); err == nil {
				t.Error("checkAPI passed a broken file")
			}
		})
	}
}

func remarshal(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
