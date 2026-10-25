package templates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// Lint runs Safety, then kubeconform for every Kubernetes version. Custom
// resources are checked against the CRD schemas in schemas (see
// catalog.CRDSchemas).
func Lint(r *Template, manifest []byte, namespace, schemas string, versions []string) error {
	if err := Safety(r, manifest, namespace); err != nil {
		return err
	}
	var errs []error
	for _, kv := range versions {
		out, err := catalog.Run(manifest, "kubeconform", "-kubernetes-version", kv, "-strict",
			"-schema-location", "default",
			"-schema-location", schemas+"/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json", "-")
		if err != nil {
			errs = append(errs, fmt.Errorf("kubeconform %s: %s", kv, strings.TrimSpace(out)))
		}
	}
	return errors.Join(errs...)
}

const selfSignedIssuer = "ci-selfsigned"

// E2EValues are the test values for one template: input values, objects to
// apply first and the smoke checks to skip with the reason. An input value is
// a text/template with .ID and .S3 (see S3). If a value uses .S3, the test
// runs SeaweedFS.
type E2EValues struct {
	Inputs    map[string]string `yaml:"inputs"`
	Setup     string            `yaml:"setup"`
	SkipSmoke map[string]string `yaml:"skipSmoke"`
}

// Usage is the idle memory and CPU of a template's pods.
type Usage struct {
	IdleMemoryMiB float64
	IdleCPUm      float64
}

// E2E installs a template and what it requires with t, runs its smoke checks
// and uninstalls everything. If measure is set, E2E also returns the idle
// usage of the pods in the template's namespace after the smoke checks.
func E2E(t *catalog.Tester, r *Template, values E2EValues, measure bool) (usage *Usage, err error) {
	ns := Namespace(r)
	for name := range values.SkipSmoke {
		if !slices.ContainsFunc(r.Smoke, func(s Smoke) bool { return s.Name == name }) {
			return nil, fmt.Errorf("skipSmoke: unknown smoke check %q", name)
		}
	}
	useS3 := slices.ContainsFunc(slices.Collect(maps.Values(values.Inputs)), func(v string) bool { return strings.Contains(v, ".S3") })
	opts := RenderOptions{Namespace: ns, Domain: r.ID + ".example.test", ClusterIssuer: selfSignedIssuer}
	// Check safety before the cluster work, with a placeholder for the gateway address.
	s3 := &S3{Endpoint: "https://s3.example.test", CIDR: "192.0.2.1/32", Bucket: "e2e"}
	render := func() ([]byte, error) {
		inputs, err := expand(r.ID, values.Inputs, s3)
		if err != nil {
			return nil, err
		}
		opts.Inputs = inputs
		if needs(r, "kubeApiCidr") && opts.Inputs["kubeApiCidr"] == "" {
			ip, err := kubectl(nil, "get", "endpointslices", "-n", "default", "-l", "kubernetes.io/service-name=kubernetes",
				"-o", "jsonpath={.items[0].endpoints[0].addresses[0]}")
			if err != nil {
				return nil, err
			}
			opts.Inputs["kubeApiCidr"] = ip + "/32"
		}
		return Render(r, opts)
	}
	manifest, err := render()
	if err != nil {
		return nil, err
	}
	if err := Safety(r, manifest, ns); err != nil {
		return nil, err
	}

	refs := r.Requires
	if useS3 {
		refs = append(slices.Clone(refs), S3Component)
	}
	uninstall, err := t.Install(refs)
	defer func() { err = errors.Join(err, uninstall()) }()
	if err != nil {
		return nil, err
	}
	if r.RequiresCertificates() {
		issuer := []byte(`{"apiVersion": "cert-manager.io/v1", "kind": "ClusterIssuer", "metadata": {"name": "` + selfSignedIssuer + `"}, "spec": {"selfSigned": {}}}`)
		if _, err := kubectl(issuer, "apply", "-f", "-"); err != nil {
			return nil, err
		}
		defer func() {
			_, e := kubectl(nil, "delete", "clusterissuer", selfSignedIssuer, "--wait", "--timeout=2m")
			err = errors.Join(err, e)
		}()
		if _, err := kubectl(nil, "wait", "clusterissuer/"+selfSignedIssuer, "--for=condition=Ready", "--timeout=3m"); err != nil {
			return nil, err
		}
	}
	if useS3 {
		var stop func() error
		if s3, stop, err = startS3(); err != nil {
			return nil, err
		}
		defer func() { err = errors.Join(err, stop()) }()
		if manifest, err = render(); err != nil {
			return nil, err
		}
	}

	// Pod Security admission applies before anything runs in the namespace.
	if err := CreateNamespace(r, ns); err != nil {
		return nil, err
	}
	defer func() {
		_, e := kubectl(nil, "delete", "namespace", ns, "--wait", "--timeout=5m")
		err = errors.Join(err, e)
	}()
	if useS3 {
		if _, err := kubectl(s3.secret(ns), "apply", "-f", "-"); err != nil {
			return nil, err
		}
	}
	if setup := strings.TrimSpace(values.Setup); setup != "" {
		if _, err := kubectl([]byte(setup), "apply", "-n", ns, "-f", "-"); err != nil {
			return nil, fmt.Errorf("setup: %w", err)
		}
	}
	secrets, err := GenerateInputs(r, ns)
	if err != nil {
		return nil, err
	}
	// kubectl refuses to apply an empty List.
	if slices.ContainsFunc(r.Inputs, func(in Input) bool { return in.Generate != "" }) {
		if _, err := kubectl(secrets, "apply", "-n", ns, "-f", "-"); err != nil {
			return nil, err
		}
	}
	if _, err := kubectl(manifest, "apply", "-n", ns, "-f", "-"); err != nil {
		return nil, err
	}
	for _, s := range r.Smoke {
		if reason, ok := values.SkipSmoke[s.Name]; ok {
			slog.Warn("smoke skipped", "template", r.ID, "check", s.Name, "reason", reason)
			continue
		}
		slog.Info("smoke", "template", r.ID, "check", s.Name)
		if _, err := kubectl(nil, "wait", "-n", ns, s.Resource, "--for="+s.Condition, "--timeout="+s.Timeout); err != nil {
			return nil, fmt.Errorf("smoke %s: %w", s.Name, err)
		}
		if s.HTTP != nil {
			err = httpSmoke(ns, s)
		}
		if s.SQL != nil {
			err = sqlSmoke(ns, s)
		}
		if err != nil {
			return nil, fmt.Errorf("smoke %s: %w", s.Name, err)
		}
	}
	// The NetworkPolicies select pods by PartOf. A pod without it, such as
	// an operator pod, has no default deny in a shared namespace.
	if out, err := kubectl(nil, "get", "pods", "-n", ns, "-l", PartOf+"!="+r.ID, "-o", "name"); err != nil {
		return nil, err
	} else if out != "" {
		return nil, fmt.Errorf("pods without %s=%s: %s", PartOf, r.ID, strings.Join(strings.Fields(out), " "))
	}
	if !measure {
		return nil, nil
	}
	slog.Info("smoke passed; waiting for idle usage to settle", "template", r.ID)
	start := time.Now()
	// The namespace is new, so every pod in it belongs to the template.
	mem, cpu, err := catalog.IdleUsage(func(podNS, _ string) bool { return podNS == ns })
	if err != nil {
		return nil, err
	}
	t.Timings.Since("settle", start)
	slog.Info("measured", "template", r.ID, "memoryMiB", int(mem), "cpuM", int(cpu))
	return &Usage{IdleMemoryMiB: mem, IdleCPUm: cpu}, nil
}

// Namespace is the namespace E2E installs r in.
func Namespace(r *Template) string { return "templates-e2e-" + r.ID }

// CreateNamespace creates ns with the Pod Security level that r needs. It
// warns at the same level, so a workload whose pods would be rejected gives a
// warning when it is applied.
func CreateNamespace(r *Template, ns string) error {
	profile := "restricted"
	for _, s := range r.Services {
		if s.SecurityProfile == "baseline" {
			profile = "baseline"
		}
	}
	if _, err := kubectl(nil, "create", "namespace", ns); err != nil {
		return err
	}
	_, err := kubectl(nil, "label", "namespace", ns, "pod-security.kubernetes.io/enforce="+profile,
		"pod-security.kubernetes.io/audit=restricted", "pod-security.kubernetes.io/warn="+profile)
	return err
}

// E2EManifest renders r for Namespace(r) with values, a placeholder S3
// gateway and a placeholder API server address.
func E2EManifest(r *Template, values E2EValues) ([]byte, error) {
	inputs, err := expand(r.ID, values.Inputs, &S3{Endpoint: "https://s3.example.test", CIDR: "192.0.2.1/32", Bucket: "e2e"})
	if err != nil {
		return nil, err
	}
	if needs(r, "kubeApiCidr") && inputs["kubeApiCidr"] == "" {
		inputs["kubeApiCidr"] = "192.0.2.2/32"
	}
	return Render(r, RenderOptions{Namespace: Namespace(r), Domain: r.ID + ".example.test", ClusterIssuer: selfSignedIssuer, Inputs: inputs})
}

// expand executes each input value as a text/template.
func expand(id string, inputs map[string]string, s3 *S3) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range inputs {
		t, err := template.New(k).Option("missingkey=error").Parse(v)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", k, err)
		}
		var b strings.Builder
		if err := t.Execute(&b, map[string]any{"ID": id, "S3": s3}); err != nil {
			return nil, fmt.Errorf("input %s: %w", k, err)
		}
		out[k] = b.String()
	}
	return out, nil
}

func needs(r *Template, input string) bool {
	for _, in := range r.Inputs {
		if in.Name == input {
			return true
		}
	}
	return false
}

func kubectl(stdin []byte, args ...string) (string, error) {
	out, err := catalog.Run(stdin, "kubectl", args...)
	return strings.TrimSpace(out), err
}

func sqlSmoke(ns string, s Smoke) error {
	query := strings.TrimSuffix(strings.TrimSpace(s.SQL.Query), ";")
	if strings.Contains(query, ";") || strings.Contains(query, "--") || strings.Contains(query, "/*") {
		return errors.New("SQL smoke takes one SELECT statement")
	}
	pods, err := kubectl(nil, "get", "pods", "-n", ns, "-l", "cnpg.io/cluster="+s.SQL.Cluster+",cnpg.io/instanceRole=primary",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return err
	}
	names := strings.Fields(pods)
	if len(names) != 1 {
		return fmt.Errorf("expected one CloudNativePG primary, found %d", len(names))
	}
	timeout, err := time.ParseDuration(s.Timeout)
	if err != nil {
		return err
	}
	got, err := kubectl(nil, "exec", "-n", ns, names[0], "-c", "postgres", "--", "psql", "-U", "postgres", "-d", s.SQL.Database,
		"-v", "ON_ERROR_STOP=1", "-qAtc", fmt.Sprintf("SET statement_timeout = %d; BEGIN READ ONLY; %s; COMMIT;", timeout.Milliseconds(), query))
	if err != nil {
		return err
	}
	if got != s.SQL.Expect {
		return fmt.Errorf("SQL result %q, want %q", got, s.SQL.Expect)
	}
	return nil
}

func httpSmoke(ns string, s Smoke) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	timeout, err := time.ParseDuration(s.Timeout)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "port-forward", "-n", ns, "service/"+s.HTTP.Service,
		fmt.Sprintf("%d:%d", port, s.HTTP.Port), "--address=127.0.0.1")
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() { cancel(); cmd.Wait() }()
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, s.HTTP.Path)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			resp.Body.Close()
			if readErr == nil && resp.StatusCode == s.HTTP.Status && strings.Contains(string(body), s.HTTP.Contains) {
				return nil
			}
			err = fmt.Errorf("HTTP %d does not match the expected status and body", resp.StatusCode)
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// ReadValues reads a JSON object of input values.
func ReadValues(path string) (map[string]string, error) {
	values := map[string]string{}
	if path == "" {
		return values, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &values); err != nil || values == nil {
		return nil, fmt.Errorf("%s: want a JSON object of strings: %v", path, err)
	}
	return values, nil
}
