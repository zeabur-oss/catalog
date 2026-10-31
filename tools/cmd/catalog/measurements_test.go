package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
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

func TestFailedRunTakesTheDistributionOutOfTestedOn(t *testing.T) {
	var cfg config
	cfg.K3s.Version = "v1.36.5+k3s1"
	comp := &catalog.Component{ID: "ingress/traefik"}
	pass := environment{IdleMemoryMiB: 22, MeasuredAt: "2026-10-30"}
	main := measurement{Component: comp.ID, Environments: map[string]environment{
		"k3s-amd64-v1.36.5+k3s1":    pass,
		"k3s-arm64-v1.36.5+k3s1":    pass,
		"single-node-amd64-v1.36.4": pass,
	}}
	for _, c := range []struct{ name, key string }{
		{"test failed", "k3s-arm64-v1.36.5+k3s1"},
		{"cluster never answered", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			// The nightly job checks out main, fails on k3s arm64 and uploads its file.
			writeFile(t, "measurements/ingress/traefik.json", main)
			if err := markFailed(cfg, "k3s", "arm64", comp, c.key); err != nil {
				t.Fatal(err)
			}
			uploaded := readMeasurement("measurements/ingress/traefik.json")
			writeFile(t, "downloaded/job/ingress/traefik.json", uploaded)
			writeFile(t, "main/ingress/traefik.json", main)
			if err := mergeMeasurements("downloaded", "main"); err != nil {
				t.Fatal(err)
			}
			merged := readMeasurement("main/ingress/traefik.json").Environments
			if got := testedOn(cfg, merged); slices.Contains(got, "k3s") {
				t.Errorf("testedOn = %q after a failed k3s run", got)
			}
			if _, ok := passed(merged)["k3s-arm64-v1.36.5+k3s1"]; ok {
				t.Error("the index publishes the failed environment as measured")
			}
			if merged["single-node-amd64-v1.36.4"].Failed || merged["k3s-amd64-v1.36.5+k3s1"].Failed {
				t.Error("the failure changed environments of other architectures or profiles")
			}

			// The next passing run replaces the mark.
			merged["k3s-arm64-v1.36.5+k3s1"] = pass
			if got := testedOn(cfg, merged); !slices.Equal(got, []string{"k3s"}) {
				t.Errorf("testedOn = %q after k3s passed again", got)
			}
		})
	}
}

func TestFailureSurvivesAnotherJobsStaleCopy(t *testing.T) {
	t.Chdir(t.TempDir())
	var cfg config
	cfg.K3s.Version = "v1.36.5+k3s1"
	comp := &catalog.Component{ID: "ingress/traefik"}
	pass := environment{IdleMemoryMiB: 22, MeasuredAt: "2026-10-30"}
	main := measurement{Component: comp.ID, Environments: map[string]environment{
		"k3s-amd64-v1.36.5+k3s1":    pass,
		"k3s-arm64-v1.36.5+k3s1":    pass,
		"single-node-amd64-v1.36.4": pass,
	}}
	// The k3s arm64 job fails; the single-node job, merged after it, uploads
	// its checkout of main with the old passing k3s measurements.
	writeFile(t, "measurements/ingress/traefik.json", main)
	if err := markFailed(cfg, "k3s", "arm64", comp, "k3s-arm64-v1.36.5+k3s1"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "downloaded/measurements-k3s-arm64-ingress-traefik/ingress/traefik.json", readMeasurement("measurements/ingress/traefik.json"))
	fresh := main
	fresh.Environments = map[string]environment{}
	for k, e := range main.Environments {
		fresh.Environments[k] = e
	}
	fresh.Environments["single-node-amd64-v1.36.4"] = environment{IdleMemoryMiB: 25, MeasuredAt: "2026-10-31"}
	writeFile(t, "downloaded/measurements-single-node-amd64-ingress-traefik/ingress/traefik.json", fresh)
	writeFile(t, "main/ingress/traefik.json", main)

	if err := mergeMeasurements("downloaded", "main"); err != nil {
		t.Fatal(err)
	}
	merged := readMeasurement("main/ingress/traefik.json").Environments
	if !merged["k3s-arm64-v1.36.5+k3s1"].Failed {
		t.Error("a stale passing copy undid the failure marker")
	}
	if merged["single-node-amd64-v1.36.4"].IdleMemoryMiB != 25 {
		t.Error("the new measurement was not merged")
	}
	if slices.Contains(testedOn(cfg, merged), "k3s") {
		t.Error("k3s stays in testedOn after its arm64 run failed")
	}
}

func TestStartupFailureWithoutMeasurementIsRecorded(t *testing.T) {
	t.Chdir(t.TempDir())
	var cfg config
	cfg.Kubesolo.Version = "v1.2.3"
	comp := &catalog.Component{ID: "ingress/traefik"}
	// amd64 passed; arm64 has never been measured and its cluster never answered.
	writeFile(t, "measurements/ingress/traefik.json", measurement{Component: comp.ID, Environments: map[string]environment{
		"kubesolo-amd64-v1.35.7+kubesolo-v1.2.3": {IdleMemoryMiB: 20, MeasuredAt: "2026-10-30"},
	}})
	if err := markFailed(cfg, kubesoloProfile, "arm64", comp, ""); err != nil {
		t.Fatal(err)
	}
	envs := readMeasurement("measurements/ingress/traefik.json").Environments
	if got := testedOn(cfg, envs); slices.Contains(got, kubesoloProfile) {
		t.Errorf("testedOn = %q after arm64 failed to start", got)
	}

	// A later arm64 run passes under its real key and replaces the mark.
	envs["kubesolo-arm64-v1.35.7+kubesolo-v1.2.3"] = environment{IdleMemoryMiB: 21, MeasuredAt: "2999-01-01"}
	if got := testedOn(cfg, envs); !slices.Equal(got, []string{kubesoloProfile}) {
		t.Errorf("testedOn = %q after arm64 passed", got)
	}
}

func TestSameDayRerunReplacesWhatMainHeld(t *testing.T) {
	t.Chdir(t.TempDir())
	// A rerun on the day whose measurements already reached main: every job
	// dates its work the same as main's, and uploads main's copy of the
	// environments it didn't measure.
	day := "2026-10-31"
	main := measurement{Component: "ingress/traefik", Environments: map[string]environment{
		"k3s-amd64-v1.36.5+k3s1":    {IdleMemoryMiB: 22, MeasuredAt: day},
		"rke2-amd64-v1.36.5+rke2r1": {Failed: true, MeasuredAt: day},
	}}
	writeFile(t, "main/ingress/traefik.json", main)
	k3s := measurement{Component: main.Component, Environments: map[string]environment{
		"k3s-amd64-v1.36.5+k3s1":    {IdleMemoryMiB: 30, MeasuredAt: day},
		"rke2-amd64-v1.36.5+rke2r1": main.Environments["rke2-amd64-v1.36.5+rke2r1"],
	}}
	rke2 := measurement{Component: main.Component, Environments: map[string]environment{
		"k3s-amd64-v1.36.5+k3s1":    main.Environments["k3s-amd64-v1.36.5+k3s1"],
		"rke2-amd64-v1.36.5+rke2r1": {IdleMemoryMiB: 40, MeasuredAt: day},
	}}
	writeFile(t, "downloaded/measurements-k3s-amd64-ingress-traefik/ingress/traefik.json", k3s)
	writeFile(t, "downloaded/measurements-rke2-amd64-ingress-traefik/ingress/traefik.json", rke2)

	if err := mergeMeasurements("downloaded", "main"); err != nil {
		t.Fatal(err)
	}
	merged := readMeasurement("main/ingress/traefik.json").Environments
	if got := merged["k3s-amd64-v1.36.5+k3s1"]; got.IdleMemoryMiB != 30 {
		t.Errorf("k3s-amd64 = %+v, want the rerun's pass, not main's copy from the rke2 job", got)
	}
	if got := merged["rke2-amd64-v1.36.5+rke2r1"]; got.Failed || got.IdleMemoryMiB != 40 {
		t.Errorf("rke2-amd64 = %+v, want the rerun's pass over main's failure", got)
	}
}
