package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// CacheDir holds pulled charts and downloaded manifests.
var CacheDir = filepath.Join(os.TempDir(), "catalog")

// Run executes a command and returns its stdout; stderr goes into the error.
func Run(stdin []byte, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// pull fetches a chart into the cache once and returns the .tgz path.
func pull(rel Release) (string, error) {
	dir := filepath.Join(CacheDir, "charts", safe(rel.Repo+rel.Chart), rel.Version)
	if m, _ := filepath.Glob(filepath.Join(dir, "*.tgz")); len(m) == 1 {
		return m[0], nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	args := []string{"pull", rel.Chart, "--version", rel.Version, "--destination", dir}
	if rel.Repo != "" {
		args = append(args, "--repo", rel.Repo)
	}
	if _, err := Run(nil, "helm", args...); err != nil {
		return "", err
	}
	m, _ := filepath.Glob(filepath.Join(dir, "*.tgz"))
	if len(m) != 1 {
		return "", fmt.Errorf("helm pull %s: expected one chart in %s", rel.Chart, dir)
	}
	return m[0], nil
}

// valuesFiles writes values documents to temporary files and returns -f
// arguments for helm, plus a cleanup function.
func valuesFiles(docs ...map[string]any) ([]string, func(), error) {
	dir, err := os.MkdirTemp("", "catalog-values-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	var args []string
	for i, d := range docs {
		data, err := yaml.Marshal(d)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		f := filepath.Join(dir, fmt.Sprintf("%d.yaml", i))
		if err := os.WriteFile(f, data, 0o644); err != nil {
			cleanup()
			return nil, nil, err
		}
		args = append(args, "-f", f)
	}
	return args, cleanup, nil
}

// releaseManifest returns a release's objects: downloaded for URLs, as written for
// inline manifests.
func releaseManifest(rel Release) ([]byte, error) {
	if rel.Kind() == "inline" {
		return []byte(rel.Manifests), nil
	}
	var buf bytes.Buffer
	for _, u := range rel.URLs {
		data, err := download(strings.ReplaceAll(u, "{version}", rel.Version))
		if err != nil {
			return nil, err
		}
		buf.Write(data)
		buf.WriteString("\n---\n")
	}
	return buf.Bytes(), nil
}

// Template renders a release for a profile with CI test inputs and extra
// values documents layered on top.
func Template(c *Component, rel Release, profile, kubeVersion string, extra ...map[string]any) ([]byte, error) {
	if rel.Kind() != "helm" {
		return releaseManifest(rel)
	}
	chart, err := pull(rel)
	if err != nil {
		return nil, err
	}
	vals, err := c.values(rel, profile, c.testInputs())
	if err != nil {
		return nil, err
	}
	files, cleanup, err := valuesFiles(append([]map[string]any{vals}, extra...)...)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	args := append([]string{"template", rel.HelmName(), chart, "--namespace", rel.Namespace,
		"--include-crds", "--kube-version", kubeVersion}, files...)
	out, err := Run(nil, "helm", args...)
	return []byte(out), err
}

func download(url string) ([]byte, error) {
	f := filepath.Join(CacheDir, "manifests", safe(url))
	if data, err := os.ReadFile(f); err == nil {
		return data, nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return nil, err
	}
	return data, os.WriteFile(f, data, 0o644)
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safe(s string) string { return unsafeChars.ReplaceAllString(s, "_") }

// ParseValues parses a values document (contributions, test overrides).
func ParseValues(doc string) (map[string]any, error) {
	m := map[string]any{}
	return m, yaml.Unmarshal([]byte(doc), &m)
}

// CRDSchemas writes a kubeconform schema for every CRD version in the YAML
// files under dir, at out/<group>/<kind>_<version>.json, so custom resources
// are checked against the exact CRDs they are installed with.
func CRDSchemas(dir, out string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, doc := range strings.Split(string(data), "\n---") {
			var crd struct {
				Kind string
				Spec struct {
					Group    string
					Names    struct{ Kind string }
					Versions []struct {
						Name   string
						Schema struct {
							OpenAPIV3Schema map[string]any `yaml:"openAPIV3Schema"`
						}
					}
				}
			}
			if yaml.Unmarshal([]byte(doc), &crd) != nil || crd.Kind != "CustomResourceDefinition" {
				continue
			}
			for _, v := range crd.Spec.Versions {
				schema := v.Schema.OpenAPIV3Schema
				for _, p := range schema["properties"].(map[string]any) {
					strict(p)
				}
				data, err := json.Marshal(schema)
				if err != nil {
					return err
				}
				f := filepath.Join(out, crd.Spec.Group, strings.ToLower(crd.Spec.Names.Kind)+"_"+v.Name+".json")
				if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(f, data, 0o644); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// strict rejects unknown fields like the API server does, and maps
// Kubernetes' int-or-string to JSON Schema.
func strict(node any) {
	s, ok := node.(map[string]any)
	if !ok {
		return
	}
	if s["x-kubernetes-int-or-string"] == true {
		delete(s, "type")
		s["oneOf"] = []any{map[string]any{"type": "string"}, map[string]any{"type": "integer"}}
	}
	if props, ok := s["properties"].(map[string]any); ok {
		if _, set := s["additionalProperties"]; !set && s["x-kubernetes-preserve-unknown-fields"] != true {
			s["additionalProperties"] = false
		}
		for _, p := range props {
			strict(p)
		}
	}
	strict(s["items"])
	strict(s["additionalProperties"])
	for _, k := range []string{"allOf", "anyOf", "oneOf"} {
		if l, ok := s[k].([]any); ok {
			for _, e := range l {
				strict(e)
			}
		}
	}
}
