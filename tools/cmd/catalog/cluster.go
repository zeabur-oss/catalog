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

// cluster is a kind cluster, or a distribution on this host, with its own
// kubeconfig. The tool never uses the contexts in the default kubeconfig.
type cluster struct {
	name, dir string
	host      string // the host profile (k3s, rke2, kubesolo); empty for kind
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
	defer os.RemoveAll(c.dir)
	switch c.host {
	case "":
		if _, err := catalog.Run(nil, "kind", "delete", "cluster", "--name", c.name,
			"--kubeconfig", filepath.Join(c.dir, "kubeconfig")); err != nil {
			slog.Warn("kind delete cluster", "name", c.name, "err", err)
		}
	// The host is a disposable runner: the uninstall scripts stop every
	// process and remove the data.
	case "k3s":
		catalog.Run(nil, "sudo", "/usr/local/bin/k3s-uninstall.sh")
	case "rke2":
		catalog.Run(nil, "sudo", "/usr/local/bin/rke2-uninstall.sh")
	case kubesoloProfile:
		catalog.Run(nil, "sudo", "systemctl", "stop", "kubesolo")
		catalog.Run(nil, "sudo", "rm", "-rf", "/var/lib/kubesolo")
	}
}

// hostProfiles install a distribution on this host. Each tests the
// components of single-node, with its values, whose distributions allow it.
var hostProfiles = []string{"k3s", "rke2", kubesoloProfile}

// hostRuns reports whether host profile p tests c.
func hostRuns(p string, c *catalog.Component) bool {
	return c.Defaults[templateProfile] != "unsupported" &&
		(len(c.Distributions) == 0 || slices.Contains(c.Distributions, p))
}

// pinnedVersion returns the release of the distribution that cfg pins for
// host profile p.
func (cfg config) pinnedVersion(p string) string {
	switch p {
	case "k3s":
		return cfg.K3s.Version
	case "rke2":
		return cfg.RKE2.Version
	case kubesoloProfile:
		return cfg.Kubesolo.Version
	}
	return ""
}

// k3sConfig is the k3s block of config/kubernetes.yaml.
type k3sConfig struct {
	Version       string            `yaml:"version"`
	SHA256        map[string]string `yaml:"sha256"` // the binary, by architecture
	InstallScript string            `yaml:"installScript"`
}

// rke2Config is the rke2 block of config/kubernetes.yaml.
type rke2Config struct {
	Version   string            `yaml:"version"`
	SHA256    map[string]string `yaml:"sha256"` // the tarball, by architecture
	LocalPath struct {
		Version string `yaml:"version"`
		SHA256  string `yaml:"sha256"`
	} `yaml:"localPath"`
}

// kubesoloConfig is the kubesolo block of config/kubernetes.yaml.
type kubesoloConfig struct {
	Version        string            `yaml:"version"`
	SHA256         map[string]string `yaml:"sha256"` // by architecture
	HelmController struct {
		Version string `yaml:"version"`
		SHA256  string `yaml:"sha256"`
	} `yaml:"helmController"`
}

// createHost installs the distribution of host profile p on this host and
// points KUBECONFIG at it. Every download must match its pinned checksum.
// needs is the install order of what the cluster tests.
func createHost(cfg config, p string, needs []*catalog.Component) (*cluster, error) {
	dir, err := os.MkdirTemp("", "catalog-"+p+"-")
	if err != nil {
		return nil, err
	}
	c := &cluster{name: p, dir: dir, host: p}
	switch p {
	case "k3s":
		err = c.startK3s(cfg.K3s)
	case "rke2":
		err = c.startRKE2(cfg.RKE2)
	case kubesoloProfile:
		err = c.startKubesolo(cfg.Kubesolo)
	default:
		err = fmt.Errorf("unknown host profile %q", p)
	}
	if err == nil {
		err = hostSetup(needs)
	}
	if err != nil {
		c.delete()
		return nil, err
	}
	return c, nil
}

// hostSetup runs the node setup of needs on this host, the only node.
func hostSetup(needs []*catalog.Component) error {
	for _, comp := range needs {
		if script, ok := nodeSetup[comp.ID]; ok {
			if _, err := catalog.Run(nil, "sudo", "sh", "-c", script); err != nil {
				return fmt.Errorf("%s node setup: %w", comp.ID, err)
			}
		}
	}
	return nil
}

// releaseURL returns the URL of file in a GitHub release. k3s and RKE2 tags
// contain a plus sign.
func releaseURL(repo, tag, file string) string {
	return "https://github.com/" + repo + "/releases/download/" + strings.ReplaceAll(tag, "+", "%2B") + "/" + file
}

// startK3s installs k3s with its install script, as the Zeabur installer
// does: the script writes the systemd unit and the uninstall script, and
// gets the checked binary. k3s's Traefik and metrics-server are off, since
// the catalog's components replace them.
func (c *cluster) startK3s(k k3sConfig) error {
	arch := runtime.GOARCH
	file := "k3s"
	if arch != "amd64" {
		file += "-" + arch
	}
	bin := filepath.Join(c.dir, "k3s")
	if err := download(releaseURL("k3s-io/k3s", k.Version, file), k.SHA256[arch], bin); err != nil {
		return err
	}
	script := filepath.Join(c.dir, "install.sh")
	if err := download("https://raw.githubusercontent.com/k3s-io/k3s/"+strings.ReplaceAll(k.Version, "+", "%2B")+"/install.sh",
		k.InstallScript, script); err != nil {
		return err
	}
	slog.Info("starting k3s", "version", k.Version)
	if _, err := catalog.Run(nil, "sudo", "install", "-m", "0755", bin, "/usr/local/bin/k3s"); err != nil {
		return err
	}
	if _, err := catalog.Run(nil, "sudo", "env", "INSTALL_K3S_SKIP_DOWNLOAD=true",
		"INSTALL_K3S_EXEC=server --disable=traefik,metrics-server", "sh", script); err != nil {
		return err
	}
	return c.waitHost("/etc/rancher/k3s/k3s.yaml", [][]string{
		{"-n", "kube-system", "wait", "--for=condition=Available", "deployment/coredns", "deployment/local-path-provisioner"},
	})
}

// startRKE2 installs RKE2 from its tarball as the RKE2 install script does,
// without its ingress controller and metrics-server, and adds
// local-path-provisioner as the default storage class.
func (c *cluster) startRKE2(r rke2Config) error {
	arch := runtime.GOARCH
	tarball := filepath.Join(c.dir, "rke2.tar.gz")
	if err := download(releaseURL("rancher/rke2", r.Version, "rke2.linux-"+arch+".tar.gz"), r.SHA256[arch], tarball); err != nil {
		return err
	}
	config := filepath.Join(c.dir, "config.yaml")
	if err := os.WriteFile(config, []byte("ingress-controller: none\ndisable: [rke2-metrics-server]\n"), 0o644); err != nil {
		return err
	}
	slog.Info("starting RKE2", "version", r.Version)
	for _, args := range [][]string{
		{"tar", "-xzf", tarball, "-C", "/usr/local"},
		{"install", "-D", "-m", "0600", config, "/etc/rancher/rke2/config.yaml"},
		{"systemctl", "daemon-reload"},
		{"systemctl", "start", "rke2-server"},
	} {
		if _, err := catalog.Run(nil, "sudo", args...); err != nil {
			return err
		}
	}
	if err := c.waitHost("/etc/rancher/rke2/rke2.yaml", [][]string{
		{"-n", "kube-system", "wait", "--for=condition=Available", "deployment/rke2-coredns-rke2-coredns"},
	}); err != nil {
		return err
	}
	manifest := filepath.Join(c.dir, "local-path-storage.yaml")
	if err := download("https://raw.githubusercontent.com/rancher/local-path-provisioner/"+r.LocalPath.Version+"/deploy/local-path-storage.yaml",
		r.LocalPath.SHA256, manifest); err != nil {
		return err
	}
	slog.Info("installing local-path-provisioner", "version", r.LocalPath.Version)
	for _, args := range [][]string{
		{"apply", "--server-side", "-f", manifest},
		{"annotate", "storageclass", "local-path", "storageclass.kubernetes.io/is-default-class=true"},
		{"-n", "local-path-storage", "rollout", "status", "deployment/local-path-provisioner", "--timeout=5m"},
	} {
		if _, err := catalog.Run(nil, "kubectl", args...); err != nil {
			return err
		}
	}
	return nil
}

// waitHost copies the distribution's admin kubeconfig, points KUBECONFIG at
// the copy and waits until the node is ready and every kubectl command in
// ready succeeds, for up to 5 minutes. The tests take their baseline of
// objects after it.
func (c *cluster) waitHost(adminKubeconfig string, ready [][]string) error {
	kubeconfig := filepath.Join(c.dir, "kubeconfig")
	os.Setenv("KUBECONFIG", kubeconfig)
	checks := append([][]string{{"wait", "--for=condition=Ready", "node", "--all"}}, ready...)
	deadline := time.Now().Add(5 * time.Minute)
	for {
		out, err := catalog.Run(nil, "sudo", "cat", adminKubeconfig)
		if err == nil {
			err = os.WriteFile(kubeconfig, []byte(out), 0o600)
		}
		for _, args := range checks {
			if err != nil {
				break
			}
			_, err = catalog.Run(nil, "kubectl", append(args, "--timeout=10s")...)
		}
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s is not ready: %w", c.host, err)
		}
		time.Sleep(5 * time.Second)
	}
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
	// KubeSolo adds CoreDNS and its storage provisioner after the node is ready.
	if err := c.waitHost("/var/lib/kubesolo/pki/admin/admin.kubeconfig", [][]string{
		{"-n", "kube-system", "wait", "--for=condition=Available", "deployment/coredns"},
		{"-n", "local-path-storage", "wait", "--for=condition=Available", "deployment/local-path-provisioner"},
	}); err != nil {
		return err
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
