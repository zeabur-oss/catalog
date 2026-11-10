package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

func writeFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := writeJSON(path, v); err != nil {
		t.Fatal(err)
	}
}

// testDigest is the digest of the source that the runs of env installed.
const testDigest = "sha256:source"

// env returns a passing run on kind, or on a host distribution at version.
func env(distribution, profile, arch, kubernetes, version string, idle int64, day string) Environment {
	return Environment{Distribution: distribution, Profile: profile, Platform: Platform{"linux", arch},
		KubernetesVersion: kubernetes, DistributionVersion: version, Result: resultPassed, SourceDigest: testDigest,
		Pods: pods(idle), MeasuredAt: day}
}

// pods is one workload whose pod uses idle MiB.
func pods(idle int64) []catalog.PodUsage {
	return []catalog.PodUsage{catalog.NewPodUsage("web", "Deployment", float64(idle), 0)}
}

func kindEnv(profile, arch string, idle int64, day string) Environment {
	return env(kindDistribution, profile, arch, "v1.36.4", "", idle, day)
}

func k3sEnv(arch string, idle int64, day string) Environment {
	return env("k3s", "single-node", arch, "v1.36.5", "v1.36.5+k3s1", idle, day)
}

func get(t *testing.T, m measurement, e Environment) Environment {
	t.Helper()
	got, ok := m.find(e)
	if !ok {
		t.Fatalf("no run on %s %s %s", e.Distribution, e.Profile, e.Platform)
	}
	return got
}

func TestMergeKeepsEnvironmentsNoJobMeasured(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFile(t, "measurements/components/traefik.json", measurement{
		Component: "traefik",
		Environments: []Environment{
			kindEnv("multi-node", "arm64", 56, "2026-10-11"),
			kindEnv("single-node", "amd64", 23, "2026-10-11"),
		},
	})
	writeFile(t, "downloaded/measurements-single-node-amd64-traefik/components/traefik.json", measurement{
		Component:    "traefik",
		Environments: []Environment{kindEnv("single-node", "amd64", 25, "2026-10-25")},
	})
	writeFile(t, "downloaded/measurements-single-node-amd64-template-ghost/templates/ghost.json", measurement{
		Template:     "ghost",
		Environments: []Environment{kindEnv("single-node", "amd64", 310, "2026-10-25")},
	})

	if err := mergeMeasurements("downloaded", "measurements"); err != nil {
		t.Fatal(err)
	}

	traefik := readMeasurement("measurements/components/traefik.json")
	if got := get(t, traefik, kindEnv("single-node", "amd64", 0, "")); got.Pods[0].Memory.Value() != 25<<20 || got.MeasuredAt != "2026-10-25" {
		t.Errorf("single-node amd64 = %+v, want the new measurement", got)
	}
	if got := get(t, traefik, kindEnv("multi-node", "arm64", 0, "")); got.Pods[0].Memory.Value() != 56<<20 {
		t.Errorf("multi-node arm64 = %+v, want it kept", got)
	}
	ghost := readMeasurement("measurements/templates/ghost.json")
	if ghost.Template != "ghost" || get(t, ghost, kindEnv("single-node", "amd64", 0, "")).Pods[0].Memory.Value() != 310<<20 {
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
		want      MeasuredVersion
	}{
		{"tag", "ghost:6.5.1", MeasuredVersion{Image: "ghost", Version: "6.5.1"}},
		{"tag and digest", "docker.io/library/ghost:6.5.1@sha256:abc", MeasuredVersion{Image: "docker.io/library/ghost", Version: "6.5.1", Digest: "sha256:abc"}},
		{"digest only", "ghcr.io/org/app@sha256:abc", MeasuredVersion{Image: "ghcr.io/org/app", Digest: "sha256:abc"}},
		{"registry port", "registry:5000/app:1.2", MeasuredVersion{Image: "registry:5000/app", Version: "1.2"}},
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
		want                   Environment
	}{
		{"kind", "multi-node", `{"serverVersion": {"gitVersion": "v1.36.4", "platform": "linux/arm64"}}`,
			Environment{Distribution: "kind", Profile: "multi-node", Platform: Platform{"linux", "arm64"}, KubernetesVersion: "v1.36.4"}},
		{"host", kubesoloProfile, `{"serverVersion": {"gitVersion": "v1.35.7+kubesolo-v1.2.3", "platform": "linux/amd64"}}`,
			Environment{Distribution: "kubesolo", Profile: "single-node", Platform: Platform{"linux", "amd64"}, KubernetesVersion: "v1.35.7", DistributionVersion: "v1.2.3"}},
		{"host release other than the pin", "k3s", `{"serverVersion": {"gitVersion": "v1.36.4+k3s1", "platform": "linux/amd64"}}`,
			Environment{Distribution: "k3s", Profile: "single-node", Platform: Platform{"linux", "amd64"}, KubernetesVersion: "v1.36.4", DistributionVersion: "v1.36.4+k3s1"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := environmentOf(cfg, c.profile, c.kubectl)
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Errorf("environmentOf = %+v, %v; want %+v", got, err, c.want)
			}
		})
	}
}

func TestFailedRunMarksOnlyItsEnvironment(t *testing.T) {
	var cfg config
	cfg.K3s.Version = "v1.36.5+k3s1"
	comp := &catalog.Component{ID: "traefik"}
	day := "2026-10-30"
	main := measurement{Component: comp.ID, Environments: []Environment{
		k3sEnv("amd64", 22, day), k3sEnv("arm64", 22, day), kindEnv("single-node", "amd64", 22, day),
	}}
	arm := k3sEnv("arm64", 0, "")
	for _, c := range []struct {
		name string
		env  *Environment
	}{
		{"test failed", &arm},
		{"cluster never answered", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			// The weekly job checks out main, fails on k3s arm64 and uploads its file.
			writeFile(t, "measurements/components/traefik.json", main)
			if err := markFailed(cfg, "k3s", "arm64", comp, c.env); err != nil {
				t.Fatal(err)
			}
			uploaded := readMeasurement("measurements/components/traefik.json")
			writeFile(t, "downloaded/job/components/traefik.json", uploaded)
			writeFile(t, "main/components/traefik.json", main)
			if err := mergeMeasurements("downloaded", "main"); err != nil {
				t.Fatal(err)
			}
			merged := readMeasurement("main/components/traefik.json")
			if got := get(t, merged, arm); got.Result != resultFailed || len(got.Pods) != 0 {
				t.Errorf("k3s arm64 = %+v after a failed run, want failed without pods", got)
			}
			if get(t, merged, kindEnv("single-node", "amd64", 0, "")).Result != resultPassed || get(t, merged, k3sEnv("amd64", 0, "")).Result != resultPassed {
				t.Error("the failure changed environments of other architectures or profiles")
			}

			// The next passing run replaces the mark.
			merged.put(k3sEnv("arm64", 22, day))
			if get(t, merged, arm).Result != resultPassed {
				t.Error("a passing run did not replace the failure")
			}
		})
	}
}

func TestFailureSurvivesAnotherJobsStaleCopy(t *testing.T) {
	t.Chdir(t.TempDir())
	var cfg config
	cfg.K3s.Version = "v1.36.5+k3s1"
	comp := &catalog.Component{ID: "traefik"}
	day := "2026-10-30"
	main := measurement{Component: comp.ID, Environments: []Environment{
		k3sEnv("amd64", 22, day), k3sEnv("arm64", 22, day), kindEnv("single-node", "amd64", 22, day),
	}}
	// The k3s arm64 job fails. The single-node job, merged after it, uploads
	// its checkout of main with the old passing k3s measurements.
	writeFile(t, "measurements/components/traefik.json", main)
	arm := k3sEnv("arm64", 0, "")
	if err := markFailed(cfg, "k3s", "arm64", comp, &arm); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "downloaded/measurements-k3s-arm64-traefik/components/traefik.json", readMeasurement("measurements/components/traefik.json"))
	fresh := measurement{Component: comp.ID, Environments: slices.Clone(main.Environments)}
	fresh.put(kindEnv("single-node", "amd64", 25, "2026-10-31"))
	writeFile(t, "downloaded/measurements-single-node-amd64-traefik/components/traefik.json", fresh)
	writeFile(t, "main/components/traefik.json", main)

	if err := mergeMeasurements("downloaded", "main"); err != nil {
		t.Fatal(err)
	}
	merged := readMeasurement("main/components/traefik.json")
	if get(t, merged, arm).Result != resultFailed {
		t.Error("a stale passing copy undid the failure marker")
	}
	if get(t, merged, kindEnv("single-node", "amd64", 0, "")).Pods[0].Memory.Value() != 25<<20 {
		t.Error("the new measurement was not merged")
	}
}

func TestStartupFailureMarksTheExistingRun(t *testing.T) {
	t.Chdir(t.TempDir())
	var cfg config
	cfg.Kubesolo.Version = "v1.2.3"
	comp := &catalog.Component{ID: "traefik"}
	writeFile(t, "measurements/components/traefik.json", measurement{Component: comp.ID, Environments: []Environment{
		env("kubesolo", "single-node", "amd64", "v1.35.7", "v1.2.3", 20, "2026-10-30"),
		env("kubesolo", "single-node", "arm64", "v1.35.7", "v1.2.3", 20, "2026-10-30"),
	}})
	if err := markFailed(cfg, kubesoloProfile, "arm64", comp, nil); err != nil {
		t.Fatal(err)
	}
	m := readMeasurement("measurements/components/traefik.json")
	arm := env("kubesolo", "single-node", "arm64", "v1.35.7", "v1.2.3", 0, "")
	amd := env("kubesolo", "single-node", "amd64", "v1.35.7", "v1.2.3", 0, "")
	if get(t, m, arm).Result != resultFailed || get(t, m, amd).Result != resultPassed {
		t.Errorf("environments = %+v after arm64 failed to start, want arm64 failed and amd64 passed", m.Environments)
	}
}

func TestStartupFailureWithoutRunRecordsNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	var cfg config
	cfg.Kubesolo.Version = "v1.2.3"
	comp := &catalog.Component{ID: "traefik"}
	if err := markFailed(cfg, kubesoloProfile, "arm64", comp, nil); err != nil {
		t.Fatal(err)
	}
	m := readMeasurement("measurements/components/traefik.json")
	if slices.ContainsFunc(m.Environments, func(e Environment) bool { return e.KubernetesVersion == "" }) {
		t.Errorf("a failure before the cluster answered wrote an environment without a Kubernetes version: %+v", m.Environments)
	}
}

func TestSameDayRerunReplacesWhatMainHeld(t *testing.T) {
	t.Chdir(t.TempDir())
	// A rerun on the day whose measurements already reached main: every job
	// dates its work the same as main's, and uploads main's copy of the
	// environments it didn't measure.
	day := "2026-10-31"
	rke2Env := func(idle int64) Environment {
		return env("rke2", "single-node", "amd64", "v1.36.5", "v1.36.5+rke2r1", idle, day)
	}
	rke2Failed := rke2Env(0)
	rke2Failed.Result, rke2Failed.Pods = resultFailed, nil
	main := measurement{Component: "traefik", Environments: []Environment{k3sEnv("amd64", 22, day), rke2Failed}}
	writeFile(t, "main/components/traefik.json", main)
	k3s := measurement{Component: main.Component, Environments: []Environment{k3sEnv("amd64", 30, day), rke2Failed}}
	rke2 := measurement{Component: main.Component, Environments: []Environment{k3sEnv("amd64", 22, day), rke2Env(40)}}
	writeFile(t, "downloaded/measurements-k3s-amd64-traefik/components/traefik.json", k3s)
	writeFile(t, "downloaded/measurements-rke2-amd64-traefik/components/traefik.json", rke2)

	if err := mergeMeasurements("downloaded", "main"); err != nil {
		t.Fatal(err)
	}
	merged := readMeasurement("main/components/traefik.json")
	if got := get(t, merged, k3sEnv("amd64", 0, "")); got.Pods[0].Memory.Value() != 30<<20 {
		t.Errorf("k3s amd64 = %+v, want the rerun's pass, not main's copy from the rke2 job", got)
	}
	if got := get(t, merged, rke2Env(0)); got.Result != resultPassed || got.Pods[0].Memory.Value() != 40<<20 {
		t.Errorf("rke2 amd64 = %+v, want the rerun's pass over main's failure", got)
	}
}

func TestBudgetReadsTheNewestKubernetesVersionOnSingleNodeKind(t *testing.T) {
	m := measurement{Environments: []Environment{
		env(kindDistribution, "single-node", "amd64", "v1.9.0", "", 10, "2026-10-30"),
		env(kindDistribution, "single-node", "amd64", "v1.36.4", "", 30, "2026-10-01"),
		env(kindDistribution, "single-node", "arm64", "v1.37.0", "", 90, "2026-10-30"),
		env("k3s", "single-node", "amd64", "v1.37.0", "v1.37.0+k3s1", 90, "2026-10-30"),
		env(kindDistribution, "multi-node", "amd64", "v1.37.0", "", 90, "2026-10-30"),
	}}
	if got, ok := latest(m); !ok || got[0].Memory.Value() != 30<<20 {
		t.Errorf("latest = %v, %v; want 30 from v1.36.4", got, ok)
	}
}

func TestMeasuredPodsRoundTripWithoutTotals(t *testing.T) {
	t.Chdir(t.TempDir())
	pods := []catalog.PodUsage{
		catalog.NewPodUsage("traefik", "Deployment", 31, 4),
		catalog.NewPodUsage("agent", "DaemonSet", 12, 1),
	}
	comp := &catalog.Component{ID: "traefik", Path: "traefik.yaml"}
	if err := os.WriteFile(comp.Path, []byte("releases: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeMeasurement(comp, k3sEnv("amd64", 0, ""), catalog.Result{Pods: pods}); err != nil {
		t.Fatal(err)
	}
	m := readMeasurement(measurementPath(comp.ID))
	want := `[{"workload":"traefik","kind":"Deployment","memory":"31Mi","cpu":"4m"},{"workload":"agent","kind":"DaemonSet","memory":"12Mi","cpu":"1m"}]`
	got, err := json.Marshal(get(t, m, k3sEnv("amd64", 0, "")).Pods)
	if err != nil || string(got) != want {
		t.Errorf("pods = %s, %v; want %s", got, err, want)
	}
	data, err := os.ReadFile(measurementPath(comp.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, total := range []string{`"idle"`, `"requests"`} {
		if strings.Contains(string(data), total) {
			t.Errorf("the file stores %s, which a reader computes", total)
		}
	}
}

func TestRunRecordsTheDigestOfTheSourceItInstalled(t *testing.T) {
	t.Chdir(t.TempDir())
	const path = "components/traefik.yaml"
	comp := &catalog.Component{ID: "traefik", Path: path}
	run := func(content string) (recorded, want string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := writeMeasurement(comp, k3sEnv("amd64", 0, ""), catalog.Result{Pods: pods(24)}); err != nil {
			t.Fatal(err)
		}
		want, err := sourceDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		return get(t, readMeasurement(measurementPath("traefik")), k3sEnv("amd64", 0, "")).SourceDigest, want
	}
	first, want := run("releases:\n- name: traefik\n  version: 42.1.3\n")
	if first != want {
		t.Errorf("sourceDigest = %q, want %q", first, want)
	}
	second, want := run("releases:\n- name: traefik\n  version: 42.1.4\n")
	if second != want || second == first {
		t.Errorf("sourceDigest = %q after the source changed, want %q", second, want)
	}
}
