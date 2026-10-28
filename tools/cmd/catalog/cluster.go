package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"go.yaml.in/yaml/v3"
)

// cluster is a kind cluster, or KubeSolo on this host, with its own
// kubeconfig. The tool never uses the contexts in the default kubeconfig.
type cluster struct {
	name, dir string
	kubesolo  bool
}

// nodeSetup holds commands that run in every node before the components that
// need them install.
var nodeSetup = map[string]string{
	// Longhorn attaches volumes over iSCSI from each node. kind's nodes ship
	// without an initiator. The iscsi_tcp module comes from the host kernel.
	"replicated-storage/longhorn": "apt-get update -qq && apt-get install -y -qq open-iscsi nfs-common && systemctl enable --now iscsid",
}

// createCluster creates a kind cluster for profile and points KUBECONFIG at
// it. needs is the install order of what the cluster tests.
func createCluster(profile, version string, needs []*catalog.Component) (*cluster, error) {
	dir, err := os.MkdirTemp("", "catalog-kind-")
	if err != nil {
		return nil, err
	}
	c := &cluster{name: "catalog-" + rand.Text()[:8], dir: dir}
	c.name = strings.ToLower(c.name)
	nodes := []map[string]any{{"role": "control-plane"}}
	if profile == "multi-node" {
		nodes = append(nodes, map[string]any{"role": "worker"}, map[string]any{"role": "worker"})
	}
	if mirror := os.Getenv("ZEABUR_REGISTRY_MIRRORS"); mirror != "" {
		hosts, err := writeMirrorHosts(filepath.Join(dir, "certs.d"), mirror)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			n["extraMounts"] = []map[string]any{{"hostPath": hosts, "containerPath": "/etc/containerd/certs.d", "readOnly": true}}
		}
	}
	cfg := map[string]any{"kind": "Cluster", "apiVersion": "kind.x-k8s.io/v1alpha4", "nodes": nodes}
	if os.Getenv("ZEABUR_REGISTRY_MIRRORS") != "" {
		// kind's containerd reads no hosts directory by default.
		cfg["containerdConfigPatches"] = []string{"[plugins.\"io.containerd.grpc.v1.cri\".registry]\n  config_path = \"/etc/containerd/certs.d\"\n"}
	}
	config, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "kind.yaml"), config, 0o644); err != nil {
		return nil, err
	}
	kubeconfig := filepath.Join(dir, "kubeconfig")
	slog.Info("creating kind cluster", "name", c.name, "profile", profile)
	if _, err := catalog.Run(nil, "kind", "create", "cluster", "--name", c.name, "--image", "kindest/node:"+version,
		"--kubeconfig", kubeconfig, "--config", filepath.Join(dir, "kind.yaml"), "--wait", "5m"); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	os.Setenv("KUBECONFIG", kubeconfig)
	if err := c.setup(needs); err != nil {
		c.delete()
		return nil, err
	}
	return c, nil
}

func (c *cluster) setup(needs []*catalog.Component) error {
	out, err := catalog.Run(nil, "kind", "get", "nodes", "--name", c.name)
	if err != nil {
		return err
	}
	for _, comp := range needs {
		script, ok := nodeSetup[comp.ID]
		if !ok {
			continue
		}
		for _, node := range strings.Fields(out) {
			if _, err := catalog.Run(nil, "docker", "exec", node, "sh", "-c", script); err != nil {
				return fmt.Errorf("%s node setup: %w", comp.ID, err)
			}
		}
	}
	return nil
}

func (c *cluster) delete() {
	if c.kubesolo {
		// The host is a disposable runner: stop the service and drop its data.
		catalog.Run(nil, "sudo", "systemctl", "stop", "kubesolo")
		catalog.Run(nil, "sudo", "rm", "-rf", "/var/lib/kubesolo")
		os.RemoveAll(c.dir)
		return
	}
	if _, err := catalog.Run(nil, "kind", "delete", "cluster", "--name", c.name,
		"--kubeconfig", filepath.Join(c.dir, "kubeconfig")); err != nil {
		slog.Warn("kind delete cluster", "name", c.name, "err", err)
	}
	os.RemoveAll(c.dir)
}

// kubesoloConfig is the kubesolo block of config/kubernetes.yaml.
type kubesoloConfig struct {
	Version        string            `yaml:"version"`
	SHA256         map[string]string `yaml:"sha256"` // by architecture
	Capabilities   []string          `yaml:"capabilities"`
	MinMemoryMiB   map[string]int    `yaml:"minMemoryMiB"` // by capability
	HelmController struct {
		Version string `yaml:"version"`
		SHA256  string `yaml:"sha256"`
	} `yaml:"helmController"`
}

// runs reports whether the personal install runs c on KubeSolo.
func (k kubesoloConfig) runs(c *catalog.Component) bool {
	return slices.Contains(k.Capabilities, c.Capability) &&
		(len(c.Distributions) == 0 || slices.Contains(c.Distributions, "kubesolo"))
}

// createKubesolo installs KubeSolo and the standalone helm-controller on this
// host and points KUBECONFIG at them. Both downloads must match their pinned
// checksums.
func createKubesolo(k kubesoloConfig) (*cluster, error) {
	dir, err := os.MkdirTemp("", "catalog-kubesolo-")
	if err != nil {
		return nil, err
	}
	c := &cluster{name: "kubesolo", dir: dir, kubesolo: true}
	if err := c.startKubesolo(k); err != nil {
		c.delete()
		return nil, err
	}
	return c, nil
}

func (c *cluster) startKubesolo(k kubesoloConfig) error {
	arch := runtime.GOARCH
	archive := filepath.Join(c.dir, "kubesolo.tar.gz")
	if err := download("https://github.com/portainer/kubesolo/releases/download/"+k.Version+"/kubesolo-"+k.Version+"-linux-"+arch+".tar.gz",
		k.SHA256[arch], archive); err != nil {
		return err
	}
	if _, err := catalog.Run(nil, "tar", "-xzf", archive, "-C", c.dir, "kubesolo"); err != nil {
		return err
	}
	slog.Info("starting KubeSolo", "version", k.Version)
	for _, args := range [][]string{
		{"install", "-m", "0755", filepath.Join(c.dir, "kubesolo"), "/usr/local/bin/kubesolo"},
		// The API server cannot reach the kubelet when the hostname maps to 127.0.1.1.
		{"sh", "-c", `sed -i '/^127\.0\.1\.1/d' /etc/hosts && echo "$(hostname -I | cut -d' ' -f1) $(hostname)" >> /etc/hosts`},
		// KillMode=process and Delegate=yes are the settings of KubeSolo's own unit.
		{"systemd-run", "--unit=kubesolo", "-p", "KillMode=process", "-p", "Delegate=yes", "-p", "OOMScoreAdjust=-500",
			"/usr/local/bin/kubesolo", "--path=/var/lib/kubesolo"},
	} {
		if _, err := catalog.Run(nil, "sudo", args...); err != nil {
			return err
		}
	}
	kubeconfig := filepath.Join(c.dir, "kubeconfig")
	os.Setenv("KUBECONFIG", kubeconfig)
	deadline := time.Now().Add(5 * time.Minute)
	for {
		out, err := catalog.Run(nil, "sudo", "cat", "/var/lib/kubesolo/pki/admin/admin.kubeconfig")
		if err == nil && os.WriteFile(kubeconfig, []byte(out), 0o600) == nil {
			// KubeSolo adds CoreDNS and its storage provisioner after the node is ready.
			// They must exist before the tests take their baseline of objects.
			if err = kubesoloReady(); err == nil {
				break
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("KubeSolo is not ready: %w", err)
		}
		time.Sleep(5 * time.Second)
	}
	manifest := filepath.Join(c.dir, "helm-controller.yaml")
	if err := download("https://github.com/k3s-io/helm-controller/releases/download/"+k.HelmController.Version+"/deploy-cluster-scoped.yaml",
		k.HelmController.SHA256, manifest); err != nil {
		return err
	}
	slog.Info("installing helm-controller", "version", k.HelmController.Version)
	if _, err := catalog.Run(nil, "kubectl", "apply", "--server-side", "-f", manifest); err != nil {
		return err
	}
	_, err := catalog.Run(nil, "kubectl", "-n", "helm-controller", "rollout", "status", "deployment/helm-controller", "--timeout=5m")
	return err
}

func kubesoloReady() error {
	for _, args := range [][]string{
		{"wait", "--for=condition=Ready", "node", "--all"},
		{"-n", "kube-system", "wait", "--for=condition=Available", "deployment/coredns"},
		{"-n", "local-path-storage", "wait", "--for=condition=Available", "deployment/local-path-provisioner"},
	} {
		if _, err := catalog.Run(nil, "kubectl", append(args, "--timeout=10s")...); err != nil {
			return err
		}
	}
	return nil
}

// download saves url to path and fails unless its SHA-256 is sum.
func download(url, sum, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return fmt.Errorf("%s: sha256 is %s, want %q", url, got, sum)
	}
	return nil
}

// writeMirrorHosts writes a containerd hosts.toml for each entry of spec, a
// comma separated list of registry=endpoint (ZEABUR_REGISTRY_MIRRORS).
// containerd falls back to the registry when the endpoint fails.
func writeMirrorHosts(dir, spec string) (string, error) {
	for _, entry := range strings.Split(spec, ",") {
		reg, endpoint, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok || reg == "" || endpoint == "" {
			return "", fmt.Errorf("ZEABUR_REGISTRY_MIRRORS: want registry=endpoint, got %q", entry)
		}
		toml := fmt.Sprintf("server = %q\n\n[host.%q]\n  capabilities = [\"pull\", \"resolve\"]\n", "https://"+reg, endpoint)
		if err := os.MkdirAll(filepath.Join(dir, reg), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, reg, "hosts.toml"), []byte(toml), 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// imagesOf returns the images in the manifests, sorted and unique.
func imagesOf(manifests ...[]byte) []string {
	seen := map[string]bool{}
	var walk func(any)
	walk = func(n any) {
		switch n := n.(type) {
		case map[string]any:
			for k, v := range n {
				if s, ok := v.(string); ok && k == "image" && s != "" && !strings.Contains(s, "{{") {
					seen[s] = true
				}
				walk(v)
			}
		case []any:
			for _, v := range n {
				walk(v)
			}
		}
	}
	for _, m := range manifests {
		for _, doc := range strings.Split(string(m), "\n---") {
			var o any
			if yaml.Unmarshal([]byte(doc), &o) == nil {
				walk(o)
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}
