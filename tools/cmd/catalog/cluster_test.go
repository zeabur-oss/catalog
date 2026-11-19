package main

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

func TestHostRuns(t *testing.T) {
	for _, c := range []struct {
		name    string
		profile string
		comp    catalog.Component
		want    bool
	}{
		{"listed alone", "rke2", catalog.Component{Distributions: []string{"rke2"}}, true},
		{"listed distribution", "k3s", catalog.Component{Distributions: []string{"k3s", "rke2"}}, true},
		{"distribution left out", "kubesolo", catalog.Component{Distributions: []string{"k3s", "rke2"}}, false},
		{"single-node unsupported", "k3s", catalog.Component{Defaults: map[string]string{"single-node": "unsupported"}}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := hostRuns(c.profile, &c.comp); got != c.want {
				t.Errorf("hostRuns(%s) = %v, want %v", c.profile, got, c.want)
			}
		})
	}
}

func TestDownloadChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("abc")) }))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "f")
	const sum = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := download(srv.URL, sum, path); err != nil {
		t.Fatalf("download with the right checksum: %v", err)
	}
	if err := download(srv.URL, "0"+sum[1:], path); err == nil {
		t.Fatal("download accepted a wrong checksum")
	}
}

func TestHelmChartJobsWaitForEveryChartsJob(t *testing.T) {
	const canal = `{"metadata": {"namespace": "kube-system", "name": "rke2-canal"}, "status": {"jobName": "helm-install-rke2-canal"}}`
	const coredns = `{"metadata": {"namespace": "kube-system", "name": "rke2-coredns"}, "status": {"jobName": "helm-install-rke2-coredns"}}`
	const snapshot = `{"metadata": {"namespace": "kube-system", "name": "rke2-snapshot-controller"}, "status": {}}`
	list := func(items ...string) []byte { return []byte(`{"items": [` + strings.Join(items, ",") + `]}`) }
	jobs, err := helmChartJobs(list(canal, coredns))
	if err != nil || !slices.Equal(jobs, []string{"kube-system/helm-install-rke2-canal", "kube-system/helm-install-rke2-coredns"}) {
		t.Errorf("helmChartJobs = %q, %v", jobs, err)
	}
	if _, err := helmChartJobs(list(canal, snapshot)); err == nil {
		t.Error("a HelmChart without its install Job counted as installed")
	}
	if jobs, err := helmChartJobs(list()); err != nil || len(jobs) != 0 {
		t.Errorf("no HelmCharts: %q, %v", jobs, err)
	}
}

func TestForwardDrops(t *testing.T) {
	if !forwardDrops("-P FORWARD DROP\n-A FORWARD -j DOCKER-USER\n") {
		t.Error("Docker's FORWARD policy not detected")
	}
	if forwardDrops("-P FORWARD ACCEPT\n-A FORWARD -m comment --comment \"-P FORWARD DROP\" -j ACCEPT\n") {
		t.Error("an ACCEPT policy reported as DROP")
	}
}

func TestHostCleanupIsValidShell(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	if out, err := exec.Command(sh, "-n", "-c", hostCleanup).CombinedOutput(); err != nil {
		t.Fatalf("sh -n: %v: %s", err, out)
	}
}

func TestContainerdProfileAllowsStackedPeer(t *testing.T) {
	for _, rule := range []string{
		"profile cri-containerd.apparmor.d ",
		"signal (send,receive) peer=cri-containerd.apparmor.d//&unconfined,",
		"ptrace (trace,tracedby,read,readby) peer=cri-containerd.apparmor.d//&unconfined,",
	} {
		if !strings.Contains(string(containerdProfile), rule) {
			t.Errorf("profile lacks %q", rule)
		}
	}
}

func TestImagesOfReadsPodsAndOperatorResources(t *testing.T) {
	manifest := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  template:
    spec:
      initContainers: [{name: init, image: registry.example/init@sha256:aa}]
      containers: [{name: web, image: registry.example/web@sha256:bb}]
---
apiVersion: dragonflydb.io/v1alpha1
kind: Dragonfly
metadata: {name: cache}
spec:
  image: registry.example/dragonfly@sha256:cc
---
apiVersion: v1
kind: ConfigMap
metadata: {name: skipped}
data: {image: "{{ .Template }}"}
`
	got, err := imagesOf([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"registry.example/dragonfly@sha256:cc", "registry.example/init@sha256:aa", "registry.example/web@sha256:bb"}
	if !slices.Equal(got, want) {
		t.Errorf("images %v, want %v", got, want)
	}
}
