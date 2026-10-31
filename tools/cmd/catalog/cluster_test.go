package main

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"slices"
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
		{"any distribution", "rke2", catalog.Component{}, true},
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
	jobs, err := helmChartJobs("kube-system rke2-canal helm-install-rke2-canal\nkube-system rke2-coredns helm-install-rke2-coredns\n")
	if err != nil || !slices.Equal(jobs, []string{"kube-system/helm-install-rke2-canal", "kube-system/helm-install-rke2-coredns"}) {
		t.Errorf("helmChartJobs = %q, %v", jobs, err)
	}
	if _, err := helmChartJobs("kube-system rke2-canal helm-install-rke2-canal\nkube-system rke2-snapshot-controller\n"); err == nil {
		t.Error("a HelmChart without its install Job counted as installed")
	}
	if jobs, err := helmChartJobs(""); err != nil || len(jobs) != 0 {
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
