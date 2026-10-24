package templates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// Lint runs Safety, then kubeconform for every Kubernetes version. Custom
// resources are checked against the CRDs of the components the template
// requires, rendered for profile.
func Lint(cat *catalog.Catalog, r *Template, manifest []byte, namespace, profile string, versions []string) error {
	if err := Safety(r, manifest, namespace); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "catalog-template-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	order, err := cat.InstallOrder(r.Requires, profile)
	if err != nil {
		return err
	}
	newest := versions[len(versions)-1]
	for _, c := range order {
		for _, rel := range c.Releases {
			out, err := catalog.Template(c, rel, profile, newest)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", c.ID, rel.Name, err)
			}
			f := filepath.Join(dir, "components", c.ID, rel.Name+".yaml")
			if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(f, out, 0o644); err != nil {
				return err
			}
		}
	}
	schemas := filepath.Join(dir, "schemas")
	if err := catalog.CRDSchemas(filepath.Join(dir, "components"), schemas); err != nil {
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

// E2E installs a template and what it requires with t, runs its smoke checks
// and uninstalls everything. setup holds objects to apply first, such as the
// Secrets that required inputs name.
func E2E(cat *catalog.Catalog, t *catalog.Tester, r *Template, inputs map[string]string, setup []byte, versions []string) (err error) {
	ns := "templates-e2e-" + r.ID
	opts := RenderOptions{Namespace: ns, Domain: r.ID + ".example.test", ClusterIssuer: selfSignedIssuer, Inputs: inputs}
	if needs(r, "kubeApiCidr") && opts.Inputs["kubeApiCidr"] == "" {
		ip, err := kubectl(nil, "get", "endpointslices", "-n", "default", "-l", "kubernetes.io/service-name=kubernetes",
			"-o", "jsonpath={.items[0].endpoints[0].addresses[0]}")
		if err != nil {
			return err
		}
		opts.Inputs["kubeApiCidr"] = ip + "/32"
	}
	manifest, err := Render(r, opts)
	if err != nil {
		return err
	}
	if err := Lint(cat, r, manifest, ns, t.Profile, versions); err != nil {
		return err
	}

	uninstall, err := t.Install(r.Requires)
	defer func() { err = errors.Join(err, uninstall()) }()
	if err != nil {
		return err
	}
	if r.RequiresCertificates() {
		issuer := []byte(`{"apiVersion": "cert-manager.io/v1", "kind": "ClusterIssuer", "metadata": {"name": "` + selfSignedIssuer + `"}, "spec": {"selfSigned": {}}}`)
		if _, err := kubectl(issuer, "apply", "-f", "-"); err != nil {
			return err
		}
		defer func() {
			_, e := kubectl(nil, "delete", "clusterissuer", selfSignedIssuer, "--wait", "--timeout=2m")
			err = errors.Join(err, e)
		}()
		if _, err := kubectl(nil, "wait", "clusterissuer/"+selfSignedIssuer, "--for=condition=Ready", "--timeout=3m"); err != nil {
			return err
		}
	}

	// Pod Security admission applies before anything runs in the namespace.
	profile := "restricted"
	for _, s := range r.Services {
		if s.SecurityProfile == "baseline" {
			profile = "baseline"
		}
	}
	if _, err := kubectl(nil, "create", "namespace", ns); err != nil {
		return err
	}
	defer func() {
		_, e := kubectl(nil, "delete", "namespace", ns, "--wait", "--timeout=5m")
		err = errors.Join(err, e)
	}()
	if _, err := kubectl(nil, "label", "namespace", ns, "pod-security.kubernetes.io/enforce="+profile,
		"pod-security.kubernetes.io/audit=restricted", "pod-security.kubernetes.io/warn=restricted"); err != nil {
		return err
	}
	if len(bytes.TrimSpace(setup)) > 0 {
		if _, err := kubectl(setup, "apply", "-n", ns, "-f", "-"); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
	}
	secrets, err := GenerateInputs(r, ns)
	if err != nil {
		return err
	}
	if _, err := kubectl(secrets, "apply", "-n", ns, "-f", "-"); err != nil {
		return err
	}
	if _, err := kubectl(manifest, "apply", "-n", ns, "-f", "-"); err != nil {
		return err
	}
	for _, s := range r.Smoke {
		slog.Info("smoke", "template", r.ID, "check", s.Name)
		if _, err := kubectl(nil, "wait", "-n", ns, s.Resource, "--for="+s.Condition, "--timeout="+s.Timeout); err != nil {
			return fmt.Errorf("smoke %s: %w", s.Name, err)
		}
		if s.HTTP != nil {
			err = httpSmoke(ns, s)
		}
		if s.SQL != nil {
			err = sqlSmoke(ns, s)
		}
		if err != nil {
			return fmt.Errorf("smoke %s: %w", s.Name, err)
		}
	}
	return nil
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
