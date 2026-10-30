package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

func writeFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := writeJSON(path, v); err != nil {
		t.Fatal(err)
	}
}

func TestMergeKeepsEnvironmentsNoJobMeasured(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFile(t, "measurements/ingress/traefik.json", measurement{
		Component: "ingress/traefik",
		Environments: map[string]environment{
			"multi-node-arm64-v1.36.4":  {IdleMemoryMiB: 56, MeasuredAt: "2026-10-11"},
			"single-node-amd64-v1.36.4": {IdleMemoryMiB: 23, MeasuredAt: "2026-10-11"},
		},
	})
	writeFile(t, "downloaded/measurements-single-node-amd64-ingress-traefik/ingress/traefik.json", measurement{
		Component:    "ingress/traefik",
		Environments: map[string]environment{"single-node-amd64-v1.36.4": {IdleMemoryMiB: 25, MeasuredAt: "2026-10-25"}},
	})
	writeFile(t, "downloaded/measurements-single-node-amd64-template-ghost/templates/ghost.json", measurement{
		Template:     "ghost",
		Environments: map[string]environment{"single-node-amd64-v1.36.4": {IdleMemoryMiB: 310, IdleCPUm: 4, MeasuredAt: "2026-10-25"}},
	})

	if err := mergeMeasurements("downloaded", "measurements"); err != nil {
		t.Fatal(err)
	}

	traefik := readMeasurement("measurements/ingress/traefik.json")
	if got := traefik.Environments["single-node-amd64-v1.36.4"]; got.IdleMemoryMiB != 25 || got.MeasuredAt != "2026-10-25" {
		t.Errorf("single-node-amd64 = %+v, want the new measurement", got)
	}
	if got := traefik.Environments["multi-node-arm64-v1.36.4"]; got.IdleMemoryMiB != 56 {
		t.Errorf("multi-node-arm64 = %+v, want it kept", got)
	}
	ghost := readMeasurement("measurements/templates/ghost.json")
	if ghost.Template != "ghost" || ghost.Environments["single-node-amd64-v1.36.4"].IdleMemoryMiB != 310 {
		t.Errorf("templates/ghost.json = %+v, want the uploaded template measurement", ghost)
	}
}

func TestMergeRejectsBrokenFile(t *testing.T) {
	t.Chdir(t.TempDir())
	path := "downloaded/job/templates/ghost.json"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeMeasurements("downloaded", "measurements"); err == nil {
		t.Fatal("merged a file that is not JSON")
	}
}

func TestIndexTemplatesCarryTheirMeasurements(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFile(t, "measurements/templates/ghost.json", measurement{
		Template:     "ghost",
		Environments: map[string]environment{"single-node-amd64-v1.36.4": {IdleMemoryMiB: 310, IdleCPUm: 4, MeasuredAt: "2026-10-25"}},
	})

	data, err := json.Marshal(indexTemplates([]*templates.Template{{ID: "ghost"}, {ID: "halo"}}))
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]json.RawMessage{}
	for _, entry := range got {
		var id string
		json.Unmarshal(entry["id"], &id)
		byID[id] = entry
	}
	var measured map[string]environment
	if err := json.Unmarshal(byID["ghost"]["measured"], &measured); err != nil {
		t.Fatalf("ghost has no measured: %v", err)
	}
	if measured["single-node-amd64-v1.36.4"].IdleMemoryMiB != 310 {
		t.Errorf("ghost measured = %+v", measured)
	}
	if _, ok := byID["halo"]["measured"]; ok {
		t.Error("halo has measured without a measurement file")
	}
	if _, ok := byID["halo"]; !ok {
		t.Error("a template without a measurement is missing from the index")
	}
}

func TestImageVersions(t *testing.T) {
	for _, c := range []struct{ name, ref, image, version string }{
		{"tag", "ghost:6.5.1", "ghost", "6.5.1"},
		{"tag and digest", "docker.io/library/ghost:6.5.1@sha256:abc", "docker.io/library/ghost", "6.5.1"},
		{"digest only", "ghcr.io/org/app@sha256:abc", "ghcr.io/org/app", "sha256:abc"},
		{"registry port", "registry:5000/app:1.2", "registry:5000/app", "1.2"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := imageVersions([]string{c.ref})[c.image]; got != c.version {
				t.Errorf("imageVersions(%q)[%q] = %q, want %q", c.ref, c.image, got, c.version)
			}
		})
	}
}

func TestTestedOnCountsOnlyThePinnedDistributionVersions(t *testing.T) {
	var cfg config
	cfg.K3s.Version = "v1.36.5+k3s1"
	cfg.RKE2.Version = "v1.36.5+rke2r1"
	cfg.Kubesolo.Version = "v1.2.3"
	for _, c := range []struct {
		name string
		keys []string
		want []string
	}{
		{"kind only", []string{"single-node-amd64-v1.36.4", "multi-node-arm64-v1.36.4"}, []string{}},
		{"pinned versions", []string{"kubesolo-arm64-v1.35.7+kubesolo-v1.2.3", "k3s-amd64-v1.36.5+k3s1", "rke2-arm64-v1.36.5+rke2r1"},
			[]string{"k3s", "rke2", "kubesolo"}},
		{"older releases", []string{"kubesolo-amd64-v1.35.7+kubesolo-v1.2.2", "k3s-amd64-v1.36.4+k3s1"}, []string{}},
		{"one architecture is enough", []string{"kubesolo-amd64-v1.35.7+kubesolo-v1.2.3", "single-node-amd64-v1.36.4"}, []string{"kubesolo"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			measured := map[string]environment{}
			for _, k := range c.keys {
				measured[k] = environment{}
			}
			got := testedOn(cfg, measured)
			if !slices.Equal(got, c.want) {
				t.Errorf("testedOn = %q, want %q", got, c.want)
			}
			// Consumers read testedOn as always present.
			if data, _ := json.Marshal(got); len(c.want) == 0 && string(data) != "[]" {
				t.Errorf("testedOn without distributions encodes as %s, want []", data)
			}
		})
	}
}
