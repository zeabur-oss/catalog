package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/zeabur-oss/catalog/tools/internal/kubeobj"
)

// CacheDir holds pulled charts and downloaded manifests.
var CacheDir = filepath.Join(os.TempDir(), "catalog")

// RunError is a command that failed. It keeps what the command wrote to stderr.
type RunError struct {
	Command string
	Err     error
	Stderr  string
}

func (e *RunError) Error() string { return fmt.Sprintf("%s: %v: %s", e.Command, e.Err, e.Stderr) }
func (e *RunError) Unwrap() error { return e.Err }

// Kubectl runs kubectl and returns its stdout without the space around it.
func Kubectl(stdin []byte, args ...string) (string, error) {
	out, err := Run(stdin, "kubectl", args...)
	return strings.TrimSpace(out), err
}

// Run executes a command and returns its stdout. A failure is a *RunError with its stderr.
func Run(stdin []byte, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), &RunError{Command: name + " " + strings.Join(args, " "), Err: err, Stderr: strings.TrimSpace(stderr.String())}
	}
	return stdout.String(), nil
}

// pull fetches a chart into the cache once and returns the .tgz path. If the
// release has a chartDigest, pull fails unless the chart matches it.
func pull(rel Release) (string, error) {
	dir := filepath.Join(CacheDir, "charts", cacheKey(rel.Repo, rel.Chart, rel.Version, rel.ChartDigest))
	m, _ := filepath.Glob(filepath.Join(dir, "*.tgz"))
	if len(m) != 1 {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		args := []string{"pull", rel.Chart, "--version", rel.Version, "--destination", dir}
		if rel.Repo != "" {
			args = append(args, "--repo", rel.Repo)
		} else if rel.ChartDigest != "" {
			// Helm refuses an OCI chart whose manifest digest differs.
			args = []string{"pull", rel.Chart + ":" + rel.Version + "@" + rel.ChartDigest, "--destination", dir}
		}
		if _, err := Run(nil, "helm", args...); err != nil {
			return "", err
		}
		if m, _ = filepath.Glob(filepath.Join(dir, "*.tgz")); len(m) != 1 {
			return "", fmt.Errorf("helm pull %s: expected one chart in %s", rel.Chart, dir)
		}
	}
	if rel.Repo != "" && rel.ChartDigest != "" {
		data, err := os.ReadFile(m[0])
		if err != nil {
			return "", err
		}
		if err := checkDigest(rel.Chart+" "+rel.Version, data, rel.ChartDigest); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
	}
	return m[0], nil
}

// checkDigest fails unless data has the SHA-256 digest want.
func checkDigest(what string, data []byte, want string) error {
	if got := fmt.Sprintf("sha256:%x", sha256.Sum256(data)); got != want {
		return fmt.Errorf("%s: the digest is %s, not %s", what, got, want)
	}
	return nil
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
	if rel.Type == "inline" {
		return []byte(rel.Manifests), nil
	}
	var buf bytes.Buffer
	for i, u := range rel.URLs {
		u = strings.ReplaceAll(u, "{version}", rel.Version)
		data, err := download(u)
		if err != nil {
			return nil, err
		}
		if i < len(rel.URLDigests) {
			if err := checkDigest(u, data, rel.URLDigests[i]); err != nil {
				return nil, err
			}
		}
		buf.Write(data)
		buf.WriteString("\n---\n")
	}
	return buf.Bytes(), nil
}

// Template renders a release for a profile with CI test inputs and extra
// values documents layered on top.
func Template(c *Component, rel Release, profile, kubeVersion string, extra ...map[string]any) ([]byte, error) {
	if rel.Type != "helm" {
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
	f := filepath.Join(CacheDir, "manifests", cacheKey(url))
	if data, err := os.ReadFile(f); err == nil {
		return data, nil
	}
	data, err := Fetch(url)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return nil, err
	}
	return data, os.WriteFile(f, data, 0o644)
}

// cacheKey names a cache entry: the SHA-256 of the parts, so it is one file
// name and different parts never share an entry.
func cacheKey(parts ...string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\x00"))))
}

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
		objs, err := kubeobj.Decode(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, o := range objs {
			if o.GetKind() != "CustomResourceDefinition" {
				continue
			}
			var crd apiextensionsv1.CustomResourceDefinition
			if err := kubeobj.Typed(o, &crd); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			for _, v := range crd.Spec.Versions {
				if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
					continue
				}
				schema := v.Schema.OpenAPIV3Schema
				for name, p := range schema.Properties {
					strict(&p)
					schema.Properties[name] = p
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
func strict(s *apiextensionsv1.JSONSchemaProps) {
	if s == nil {
		return
	}
	if s.XIntOrString {
		s.Type = ""
		s.OneOf = []apiextensionsv1.JSONSchemaProps{{Type: "string"}, {Type: "integer"}}
	}
	if len(s.Properties) > 0 && s.AdditionalProperties == nil && (s.XPreserveUnknownFields == nil || !*s.XPreserveUnknownFields) {
		s.AdditionalProperties = &apiextensionsv1.JSONSchemaPropsOrBool{Allows: false}
	}
	for name, p := range s.Properties {
		strict(&p)
		s.Properties[name] = p
	}
	if s.Items != nil {
		strict(s.Items.Schema)
		for i := range s.Items.JSONSchemas {
			strict(&s.Items.JSONSchemas[i])
		}
	}
	if s.AdditionalProperties != nil {
		strict(s.AdditionalProperties.Schema)
	}
	for _, list := range [][]apiextensionsv1.JSONSchemaProps{s.AllOf, s.AnyOf, s.OneOf} {
		for i := range list {
			strict(&list[i])
		}
	}
}

// PinnedImage is an image reference with a sha256 digest.
var PinnedImage = regexp.MustCompile(`^[^\s]+@sha256:[a-f0-9]{64}$`)
