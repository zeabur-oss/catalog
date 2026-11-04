package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

func writeFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := writeJSON(path, v); err != nil {
		t.Fatal(err)
	}
}

// env returns a passing run on kind, or on a host distribution at version.
func env(distribution, profile, arch, kubernetes, version string, idle int64, day string) environment {
	return environment{Distribution: distribution, Profile: profile, Platform: platform{"linux", arch},
		KubernetesVersion: kubernetes, DistributionVersion: version, Result: resultPassed,
		Idle: &catalog.Amounts{MemoryMiB: idle}, MeasuredAt: day}
}

func kindEnv(profile, arch string, idle int64, day string) environment {
	return env(kindDistribution, profile, arch, "v1.36.4", "", idle, day)
}

func k3sEnv(arch string, idle int64, day string) environment {
	return env("k3s", "single-node", arch, "v1.36.5", "v1.36.5+k3s1", idle, day)
}

func get(t *testing.T, m measurement, e environment) environment {
	t.Helper()
	got, ok := m.find(e)
	if !ok {
		t.Fatalf("no run on %s %s %s", e.Distribution, e.Profile, e.Platform)
	}
	return got
}

func TestMergeKeepsEnvironmentsNoJobMeasured(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFile(t, "measurements/components/ingress/traefik.json", measurement{
		Component: "ingress/traefik",
		Environments: []environment{
			kindEnv("multi-node", "arm64", 56, "2026-10-11"),
			kindEnv("single-node", "amd64", 23, "2026-10-11"),
		},
	})
	writeFile(t, "downloaded/measurements-single-node-amd64-ingress-traefik/components/ingress/traefik.json", measurement{
		Component:    "ingress/traefik",
		Environments: []environment{kindEnv("single-node", "amd64", 25, "2026-10-25")},
	})
	writeFile(t, "downloaded/measurements-single-node-amd64-template-ghost/templates/ghost.json", measurement{
		Template:     "ghost",
		Environments: []environment{kindEnv("single-node", "amd64", 310, "2026-10-25")},
	})

	if err := mergeMeasurements("downloaded", "measurements"); err != nil {
		t.Fatal(err)
	}

	traefik := readMeasurement("measurements/components/ingress/traefik.json")
	if got := get(t, traefik, kindEnv("single-node", "amd64", 0, "")); got.Idle.MemoryMiB != 25 || got.MeasuredAt != "2026-10-25" {
		t.Errorf("single-node amd64 = %+v, want the new measurement", got)
	}
	if got := get(t, traefik, kindEnv("multi-node", "arm64", 0, "")); got.Idle.MemoryMiB != 56 {
		t.Errorf("multi-node arm64 = %+v, want it kept", got)
	}
	ghost := readMeasurement("measurements/templates/ghost.json")
	if ghost.Template != "ghost" || get(t, ghost, kindEnv("single-node", "amd64", 0, "")).Idle.MemoryMiB != 310 {
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

func TestImageVersions(t *testing.T) {
	for _, c := range []struct {
		name, ref string
		want      measuredVersion
	}{
		{"tag", "ghost:6.5.1", measuredVersion{Image: "ghost", Version: "6.5.1"}},
		{"tag and digest", "docker.io/library/ghost:6.5.1@sha256:abc", measuredVersion{Image: "docker.io/library/ghost", Version: "6.5.1", Digest: "sha256:abc"}},
		{"digest only", "ghcr.io/org/app@sha256:abc", measuredVersion{Image: "ghcr.io/org/app", Digest: "sha256:abc"}},
		{"registry port", "registry:5000/app:1.2", measuredVersion{Image: "registry:5000/app", Version: "1.2"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := imageVersions([]string{c.ref}); len(got) != 1 || got[0] != c.want {
				t.Errorf("imageVersions(%q) = %+v, want %+v", c.ref, got, c.want)
			}
		})
	}
}

func TestEnvironmentOfNamesTheDistributionApartFromTheProfile(t *testing.T) {
	var cfg config
	cfg.Kubesolo.Version = "v1.2.3"
	cfg.K3s.Version = "v1.36.5+k3s1"
	for _, c := range []struct {
		name, profile, kubectl string
		want                   environment
	}{
		{"kind", "multi-node", `{"serverVersion": {"gitVersion": "v1.36.4", "platform": "linux/arm64"}}`,
			environment{Distribution: "kind", Profile: "multi-node", Platform: platform{"linux", "arm64"}, KubernetesVersion: "v1.36.4"}},
		{"host", kubesoloProfile, `{"serverVersion": {"gitVersion": "v1.35.7+kubesolo-v1.2.3", "platform": "linux/amd64"}}`,
			environment{Distribution: "kubesolo", Profile: "single-node", Platform: platform{"linux", "amd64"}, KubernetesVersion: "v1.35.7", DistributionVersion: "v1.2.3"}},
		{"host release other than the pin", "k3s", `{"serverVersion": {"gitVersion": "v1.36.4+k3s1", "platform": "linux/amd64"}}`,
			environment{Distribution: "k3s", Profile: "single-node", Platform: platform{"linux", "amd64"}, KubernetesVersion: "v1.36.4", DistributionVersion: "v1.36.4+k3s1"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := environmentOf(cfg, c.profile, c.kubectl)
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Errorf("environmentOf = %+v, %v; want %+v", got, err, c.want)
			}
		})
	}
}

func TestTestedOnCountsOnlyThePinnedDistributionVersions(t *testing.T) {
	var cfg config
	cfg.K3s.Version = "v1.36.5+k3s1"
	cfg.RKE2.Version = "v1.36.5+rke2r1"
	cfg.Kubesolo.Version = "v1.2.3"
	day := "2026-10-30"
	for _, c := range []struct {
		name string
		envs []environment
		want []tested
	}{
		{"kind only", []environment{kindEnv("single-node", "amd64", 1, day), kindEnv("multi-node", "arm64", 1, day)}, []tested{}},
		{"pinned versions", []environment{
			env("kubesolo", "single-node", "arm64", "v1.35.7", "v1.2.3", 1, day),
			k3sEnv("amd64", 1, day),
			env("rke2", "single-node", "arm64", "v1.36.5", "v1.36.5+rke2r1", 1, day),
		}, []tested{{"k3s", []string{"linux/amd64"}}, {"rke2", []string{"linux/arm64"}}, {"kubesolo", []string{"linux/arm64"}}}},
		{"older releases", []environment{
			env("kubesolo", "single-node", "amd64", "v1.35.7", "v1.2.2", 1, day),
			env("k3s", "single-node", "amd64", "v1.36.4", "v1.36.4+k3s1", 1, day),
		}, []tested{}},
		{"each platform that passed", []environment{k3sEnv("amd64", 1, day), k3sEnv("arm64", 1, day)},
			[]tested{{"k3s", []string{"linux/amd64", "linux/arm64"}}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := testedOn(cfg, c.envs)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("testedOn = %+v, want %+v", got, c.want)
			}
			// Consumers read testedOn as always present.
			if data, _ := json.Marshal(got); len(c.want) == 0 && string(data) != "[]" {
				t.Errorf("testedOn without distributions encodes as %s, want []", data)
			}
		})
	}
}

// k3sPlatforms returns the platforms that testedOn lists for k3s.
func k3sPlatforms(cfg config, envs []environment) []string {
	for _, d := range testedOn(cfg, envs) {
		if d.Distribution == "k3s" {
			return d.Platforms
		}
	}
	return nil
}

func TestFailedRunTakesThePlatformOutOfTestedOn(t *testing.T) {
	var cfg config
	cfg.K3s.Version = "v1.36.5+k3s1"
	comp := &catalog.Component{ID: "ingress/traefik"}
	day := "2026-10-30"
	main := measurement{Component: comp.ID, Environments: []environment{
		k3sEnv("amd64", 22, day), k3sEnv("arm64", 22, day), kindEnv("single-node", "amd64", 22, day),
	}}
	arm := k3sEnv("arm64", 0, "")
	for _, c := range []struct {
		name string
		env  *environment
	}{
		{"test failed", &arm},
		{"cluster never answered", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			// The nightly job checks out main, fails on k3s arm64 and uploads its file.
			writeFile(t, "measurements/components/ingress/traefik.json", main)
			if err := markFailed(cfg, "k3s", "arm64", comp, c.env); err != nil {
				t.Fatal(err)
			}
			uploaded := readMeasurement("measurements/components/ingress/traefik.json")
			writeFile(t, "downloaded/job/components/ingress/traefik.json", uploaded)
			writeFile(t, "main/components/ingress/traefik.json", main)
			if err := mergeMeasurements("downloaded", "main"); err != nil {
				t.Fatal(err)
			}
			merged := readMeasurement("main/components/ingress/traefik.json")
			if got := k3sPlatforms(cfg, merged.Environments); !slices.Equal(got, []string{"linux/amd64"}) {
				t.Errorf("k3s platforms = %q after a failed arm64 run, want amd64 only", got)
			}
			if slices.ContainsFunc(passed(merged.Environments), arm.same) {
				t.Error("the index publishes the failed environment as measured")
			}
			if get(t, merged, kindEnv("single-node", "amd64", 0, "")).Result != resultPassed || get(t, merged, k3sEnv("amd64", 0, "")).Result != resultPassed {
				t.Error("the failure changed environments of other architectures or profiles")
			}

			// The next passing run replaces the mark.
			merged.put(k3sEnv("arm64", 22, day))
			if got := k3sPlatforms(cfg, merged.Environments); !slices.Equal(got, []string{"linux/amd64", "linux/arm64"}) {
				t.Errorf("k3s platforms = %q after arm64 passed again", got)
			}
		})
	}
}

func TestFailureSurvivesAnotherJobsStaleCopy(t *testing.T) {
	t.Chdir(t.TempDir())
	var cfg config
	cfg.K3s.Version = "v1.36.5+k3s1"
	comp := &catalog.Component{ID: "ingress/traefik"}
	day := "2026-10-30"
	main := measurement{Component: comp.ID, Environments: []environment{
		k3sEnv("amd64", 22, day), k3sEnv("arm64", 22, day), kindEnv("single-node", "amd64", 22, day),
	}}
	// The k3s arm64 job fails. The single-node job, merged after it, uploads
	// its checkout of main with the old passing k3s measurements.
	writeFile(t, "measurements/components/ingress/traefik.json", main)
	arm := k3sEnv("arm64", 0, "")
	if err := markFailed(cfg, "k3s", "arm64", comp, &arm); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "downloaded/measurements-k3s-arm64-ingress-traefik/components/ingress/traefik.json", readMeasurement("measurements/components/ingress/traefik.json"))
	fresh := measurement{Component: comp.ID, Environments: slices.Clone(main.Environments)}
	fresh.put(kindEnv("single-node", "amd64", 25, "2026-10-31"))
	writeFile(t, "downloaded/measurements-single-node-amd64-ingress-traefik/components/ingress/traefik.json", fresh)
	writeFile(t, "main/components/ingress/traefik.json", main)

	if err := mergeMeasurements("downloaded", "main"); err != nil {
		t.Fatal(err)
	}
	merged := readMeasurement("main/components/ingress/traefik.json")
	if get(t, merged, arm).Result != resultFailed {
		t.Error("a stale passing copy undid the failure marker")
	}
	if get(t, merged, kindEnv("single-node", "amd64", 0, "")).Idle.MemoryMiB != 25 {
		t.Error("the new measurement was not merged")
	}
	if got := k3sPlatforms(cfg, merged.Environments); slices.Contains(got, "linux/arm64") {
		t.Error("k3s arm64 stays in testedOn after its run failed")
	}
}

func TestStartupFailureWithoutMeasurementIsRecorded(t *testing.T) {
	t.Chdir(t.TempDir())
	var cfg config
	cfg.Kubesolo.Version = "v1.2.3"
	comp := &catalog.Component{ID: "ingress/traefik"}
	// amd64 passed. arm64 has never been measured and its cluster never answered.
	writeFile(t, "measurements/components/ingress/traefik.json", measurement{Component: comp.ID, Environments: []environment{
		env("kubesolo", "single-node", "amd64", "v1.35.7", "v1.2.3", 20, "2026-10-30"),
		env("kubesolo", "single-node", "arm64", "v1.35.7", "v1.2.2", 20, "2026-10-30"),
	}})
	if err := markFailed(cfg, kubesoloProfile, "arm64", comp, nil); err != nil {
		t.Fatal(err)
	}
	m := readMeasurement("measurements/components/ingress/traefik.json")
	if got := testedOn(cfg, m.Environments); !reflect.DeepEqual(got, []tested{{"kubesolo", []string{"linux/amd64"}}}) {
		t.Errorf("testedOn = %+v after arm64 failed to start, want amd64 only", got)
	}

	// A later arm64 run passes under its real Kubernetes version and replaces the mark.
	m.put(env("kubesolo", "single-node", "arm64", "v1.35.7", "v1.2.3", 21, "2999-01-01"))
	if got := testedOn(cfg, m.Environments); !reflect.DeepEqual(got, []tested{{"kubesolo", []string{"linux/amd64", "linux/arm64"}}}) {
		t.Errorf("testedOn = %+v after arm64 passed", got)
	}
}

func TestSameDayRerunReplacesWhatMainHeld(t *testing.T) {
	t.Chdir(t.TempDir())
	// A rerun on the day whose measurements already reached main: every job
	// dates its work the same as main's, and uploads main's copy of the
	// environments it didn't measure.
	day := "2026-10-31"
	rke2Env := func(idle int64) environment {
		return env("rke2", "single-node", "amd64", "v1.36.5", "v1.36.5+rke2r1", idle, day)
	}
	rke2Failed := rke2Env(0)
	rke2Failed.Result, rke2Failed.Idle = resultFailed, nil
	main := measurement{Component: "ingress/traefik", Environments: []environment{k3sEnv("amd64", 22, day), rke2Failed}}
	writeFile(t, "main/components/ingress/traefik.json", main)
	k3s := measurement{Component: main.Component, Environments: []environment{k3sEnv("amd64", 30, day), rke2Failed}}
	rke2 := measurement{Component: main.Component, Environments: []environment{k3sEnv("amd64", 22, day), rke2Env(40)}}
	writeFile(t, "downloaded/measurements-k3s-amd64-ingress-traefik/components/ingress/traefik.json", k3s)
	writeFile(t, "downloaded/measurements-rke2-amd64-ingress-traefik/components/ingress/traefik.json", rke2)

	if err := mergeMeasurements("downloaded", "main"); err != nil {
		t.Fatal(err)
	}
	merged := readMeasurement("main/components/ingress/traefik.json")
	if got := get(t, merged, k3sEnv("amd64", 0, "")); got.Idle.MemoryMiB != 30 {
		t.Errorf("k3s amd64 = %+v, want the rerun's pass, not main's copy from the rke2 job", got)
	}
	if got := get(t, merged, rke2Env(0)); got.Result != resultPassed || got.Idle.MemoryMiB != 40 {
		t.Errorf("rke2 amd64 = %+v, want the rerun's pass over main's failure", got)
	}
}

func TestBudgetReadsTheNewestKubernetesVersionOnSingleNodeKind(t *testing.T) {
	m := measurement{Environments: []environment{
		env(kindDistribution, "single-node", "amd64", "v1.9.0", "", 10, "2026-10-30"),
		env(kindDistribution, "single-node", "amd64", "v1.36.4", "", 30, "2026-10-01"),
		env(kindDistribution, "single-node", "arm64", "v1.37.0", "", 90, "2026-10-30"),
		env("k3s", "single-node", "amd64", "v1.37.0", "v1.37.0+k3s1", 90, "2026-10-30"),
		env(kindDistribution, "multi-node", "amd64", "v1.37.0", "", 90, "2026-10-30"),
	}}
	if got, ok := latest(m); !ok || got != 30 {
		t.Errorf("latest = %d, %v; want 30 from v1.36.4", got, ok)
	}
}

func TestRequestMemoryIsPublishedWhenMeasuredAndOmittedWhenNot(t *testing.T) {
	idle := &catalog.Amounts{MemoryMiB: 50}
	for _, c := range []struct {
		env  environment
		want any
	}{
		{environment{Idle: idle, Requests: &catalog.Amounts{MemoryMiB: 96}}, map[string]any{"memoryMiB": float64(96)}},
		// No pod asks for memory.
		{environment{Idle: idle, Requests: &catalog.Amounts{}}, map[string]any{}},
		// Measured before requests were recorded.
		{environment{Idle: idle}, nil},
	} {
		out, err := json.Marshal(c.env)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got["requests"], c.want) {
			t.Errorf("%s: requests = %v, want %v", out, got["requests"], c.want)
		}
	}
}
