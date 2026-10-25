package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

func TestKubesoloRuns(t *testing.T) {
	k := kubesoloConfig{Capabilities: []string{"ingress", "databases"}}
	for _, c := range []struct {
		name string
		comp catalog.Component
		want bool
	}{
		{"listed capability", catalog.Component{Capability: "ingress"}, true},
		{"other capability", catalog.Component{Capability: "logs"}, false},
		{"distribution without kubesolo", catalog.Component{Capability: "databases", Distributions: []string{"k3s", "rke2"}}, false},
		{"distribution with kubesolo", catalog.Component{Capability: "databases", Distributions: []string{"kubesolo"}}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := k.runs(&c.comp); got != c.want {
				t.Errorf("runs = %v, want %v", got, c.want)
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
