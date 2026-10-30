package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
