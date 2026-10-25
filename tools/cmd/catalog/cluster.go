package main

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"go.yaml.in/yaml/v3"
)

// cluster is a kind cluster with its own kubeconfig. The tool never uses the
// contexts in the default kubeconfig.
type cluster struct {
	name, dir string
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
	// Templates name the local-path StorageClass. kind's default StorageClass
	// uses the same provisioner.
	sc := `{"apiVersion": "storage.k8s.io/v1", "kind": "StorageClass", "metadata": {"name": "local-path"},
		"provisioner": "rancher.io/local-path", "reclaimPolicy": "Delete", "volumeBindingMode": "WaitForFirstConsumer"}`
	if _, err := catalog.Run([]byte(sc), "kubectl", "apply", "-f", "-"); err != nil {
		return err
	}
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
	if _, err := catalog.Run(nil, "kind", "delete", "cluster", "--name", c.name,
		"--kubeconfig", filepath.Join(c.dir, "kubeconfig")); err != nil {
		slog.Warn("kind delete cluster", "name", c.name, "err", err)
	}
	os.RemoveAll(c.dir)
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
