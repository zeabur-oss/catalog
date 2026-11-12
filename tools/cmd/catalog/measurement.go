package main

import (
	"cmp"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/sumdb/dirhash"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// measurement is measurements/components/<id>.json for a
// component and measurements/templates/<id>.json for a template.
type measurement struct {
	Component    string        `json:"component,omitempty"`
	Template     string        `json:"template,omitempty"`
	Environments []Environment `json:"environments"`
}

// Results of a run. A reader treats any other value as not passed.
const (
	resultPassed = "passed"
	resultFailed = "failed"
)

// kindDistribution is the distribution of a kind cluster.
const kindDistribution = "kind"

// Platform is the OS and architecture of the nodes of a run.
type Platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

func (p Platform) String() string { return p.OS + "/" + p.Architecture }

// Environment is one run of a component or template test. distribution, profile, platform,
// kubernetesVersion and distributionVersion identify it.
type Environment struct {
	// kind, k3s, rke2 or kubesolo. A reader ignores a value it does not know.
	Distribution string `json:"distribution"`
	// The catalog profile whose values the run installed.
	Profile  string   `json:"profile"`
	Platform Platform `json:"platform"`
	// Empty when the cluster never answered.
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// The release of a distribution installed on the host. kind has none.
	DistributionVersion string `json:"distributionVersion,omitempty"`
	// passed or failed. A reader treats any other value as not passed.
	Result string `json:"result"`
	// The idle usage of one pod of each workload the run started.
	Pods []catalog.PodUsage `json:"pods,omitempty"`
	// The version of each release that the run installed.
	Versions []MeasuredVersion `json:"versions,omitempty"`
	// The Go module hash (h1:) of the component file, or of the template directory, that the run
	// installed.
	SourceDigest string `json:"sourceDigest,omitempty"`
	// The sourceDigest of each component that the run installed first, by ID.
	Dependencies map[string]string `json:"dependencies,omitempty"`
	// The pinned release of each add-on that the run installed on the host, by name.
	AddOns map[string]string `json:"addOns,omitempty"`
	// The day of the run, in UTC.
	MeasuredAt string `json:"measuredAt" jsonschema:"format=date"`
}

// MeasuredVersion is the version of a component's release or of a template's image.
type MeasuredVersion struct {
	Release string `json:"release,omitempty"`
	Image   string `json:"image,omitempty"`
	Version string `json:"version,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

// sameInputs reports whether e ran with the inputs of want, the result of inputs. A run on a host
// distribution reports the Kubernetes release of its distributionVersion, which want leaves empty.
func (e Environment) sameInputs(want Environment) bool {
	return e.Distribution == want.Distribution && e.Profile == want.Profile && e.Platform == want.Platform &&
		(want.KubernetesVersion == "" || e.KubernetesVersion == want.KubernetesVersion) &&
		e.DistributionVersion == want.DistributionVersion && e.SourceDigest == want.SourceDigest &&
		maps.Equal(e.Dependencies, want.Dependencies) && maps.Equal(e.AddOns, want.AddOns)
}

// same reports whether e and o are runs in the same environment.
func (e Environment) same(o Environment) bool {
	return e.Distribution == o.Distribution && e.Profile == o.Profile && e.Platform == o.Platform &&
		e.KubernetesVersion == o.KubernetesVersion && e.DistributionVersion == o.DistributionVersion
}

func compareEnvironments(a, b Environment) int {
	return cmp.Or(cmp.Compare(a.Distribution, b.Distribution), cmp.Compare(a.Profile, b.Profile),
		cmp.Compare(a.Platform.String(), b.Platform.String()), cmp.Compare(a.KubernetesVersion, b.KubernetesVersion),
		cmp.Compare(a.DistributionVersion, b.DistributionVersion))
}

// find returns the run in the environment of e, if any.
func (m *measurement) find(e Environment) (Environment, bool) {
	i := slices.IndexFunc(m.Environments, e.same)
	if i < 0 {
		return Environment{}, false
	}
	return m.Environments[i], true
}

// put replaces the run in the environment of e, or adds e.
func (m *measurement) put(e Environment) {
	if i := slices.IndexFunc(m.Environments, e.same); i >= 0 {
		m.Environments[i] = e
		return
	}
	m.Environments = append(m.Environments, e)
}

// environmentOf returns the environment of a cluster from the output of
// kubectl version -o json. profile is the -profile of the run.
func environmentOf(cfg config, profile, kubectlVersion string) (Environment, error) {
	var v struct {
		ServerVersion struct{ GitVersion, Platform string }
	}
	if err := json.Unmarshal([]byte(kubectlVersion), &v); err != nil {
		return Environment{}, err
	}
	e := hostEnvironment(cfg, profile, "")
	e.Platform.OS, e.Platform.Architecture, _ = strings.Cut(v.ServerVersion.Platform, "/") // linux/amd64
	// v1.36.5+k3s1 and v1.35.7+kubesolo-v1.2.3 carry the distribution's release after the plus.
	git := v.ServerVersion.GitVersion
	e.KubernetesVersion, _, _ = strings.Cut(git, "+")
	if e.DistributionVersion != "" {
		// The cluster can run another release than the pin, such as with -cluster current.
		e.DistributionVersion = git
		if _, release, ok := strings.Cut(git, "+kubesolo-"); ok {
			e.DistributionVersion = release
		}
	}
	return e, nil
}

// runEnvironment returns the distribution, profile and platform of a run of
// profile on linux/arch. A host profile runs the single-node values.
func runEnvironment(profile, arch string) Environment {
	e := Environment{Distribution: kindDistribution, Profile: profile, Platform: Platform{"linux", arch}}
	if slices.Contains(hostProfiles, profile) {
		e.Distribution, e.Profile = profile, templateProfile
	}
	return e
}

// hostEnvironment is runEnvironment with the pinned release of a host
// profile's distribution.
func hostEnvironment(cfg config, profile, arch string) Environment {
	e := runEnvironment(profile, arch)
	e.DistributionVersion = cfg.pinnedVersion(profile)
	return e
}

func measurementPath(id string) string {
	return filepath.Join("measurements", "components", id+".json")
}

func templateMeasurementPath(id string) string {
	return filepath.Join("measurements", "templates", id+".json")
}

// sourceDigest returns the Go module hash (dirhash.Hash1, "h1:...") of the
// file at path, or of every file under it when it is a directory.
func sourceDigest(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return dirhash.HashDir(path, "", dirhash.Hash1)
	}
	open := func(string) (io.ReadCloser, error) { return os.Open(path) }
	return dirhash.Hash1([]string{filepath.Base(path)}, open)
}

func today() string { return time.Now().UTC().Format(time.DateOnly) }

func releaseVersions(c *catalog.Component) []MeasuredVersion {
	var out []MeasuredVersion
	for _, r := range c.Releases {
		if r.Version != "" {
			out = append(out, MeasuredVersion{Release: r.Name, Version: r.Version})
		}
	}
	return out
}

// writeMeasurement records the usage of c in env, a run with the source
// digests of in, the result of inputs.
func writeMeasurement(c *catalog.Component, env, in Environment, res catalog.Result) error {
	m := readMeasurement(measurementPath(c.ID))
	m.Component = c.ID
	env.Result = resultPassed
	env.Pods = res.Pods
	env.Versions = releaseVersions(c)
	env.SourceDigest, env.Dependencies, env.AddOns = in.SourceDigest, in.Dependencies, in.AddOns
	env.MeasuredAt = today()
	m.put(env)
	return writeMeasurementFile(measurementPath(c.ID), m)
}

// markFailed records that the test of c failed in env, or before the cluster
// answered when env is nil. On a host profile it also marks every run of the
// pinned release on linux/arch, so a failure before the cluster answered
// counts too. With no such run, it records nothing, because a component with
// no run already shows as untested. in holds the source digests of the run.
func markFailed(cfg config, profile, arch string, c *catalog.Component, env *Environment, in Environment) error {
	m := readMeasurement(measurementPath(c.ID))
	m.Component = c.ID
	failed := func(e Environment) Environment {
		return Environment{Distribution: e.Distribution, Profile: e.Profile, Platform: e.Platform,
			KubernetesVersion: e.KubernetesVersion, DistributionVersion: e.DistributionVersion,
			Result: resultFailed, Versions: releaseVersions(c), SourceDigest: in.SourceDigest,
			Dependencies: in.Dependencies, AddOns: in.AddOns, MeasuredAt: today()}
	}
	if env != nil {
		m.put(failed(*env))
	}
	if slices.Contains(hostProfiles, profile) {
		host := hostEnvironment(cfg, profile, arch)
		for i, e := range m.Environments {
			if e.Distribution == host.Distribution && e.Platform == host.Platform && e.DistributionVersion == host.DistributionVersion {
				m.Environments[i] = failed(e)
			}
		}
	}
	return writeMeasurementFile(measurementPath(c.ID), m)
}

// writeTemplateMeasurement records the usage of tmpl in env, with the
// images in its manifest, and the source digests of in.
func writeTemplateMeasurement(tmpl *templates.Template, env, in Environment, pods []catalog.PodUsage, images []string) error {
	id := tmpl.ID
	path := templateMeasurementPath(id)
	m := readMeasurement(path)
	m.Template = id
	env.Result = resultPassed
	env.Pods = pods
	env.Versions = imageVersions(images)
	env.SourceDigest, env.Dependencies, env.AddOns = in.SourceDigest, in.Dependencies, in.AddOns
	env.MeasuredAt = today()
	m.put(env)
	return writeMeasurementFile(path, m)
}

// imageVersions returns the tag and the digest of each image reference.
func imageVersions(images []string) []MeasuredVersion {
	var out []MeasuredVersion
	for _, ref := range images {
		name, digest, _ := strings.Cut(ref, "@")
		v := MeasuredVersion{Image: name, Digest: digest}
		// A colon after the last slash starts a tag. A colon before it is a registry port.
		if i := strings.LastIndex(name, ":"); i > strings.LastIndex(name, "/") {
			v.Image, v.Version = name[:i], name[i+1:]
		}
		out = append(out, v)
	}
	return out
}

// readMeasurement returns the file at path, or an empty measurement when it
// is missing or not in the current format.
func readMeasurement(path string) measurement {
	m := measurement{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &m)
	}
	return m
}

func writeMeasurementFile(path string, m measurement) error {
	slices.SortFunc(m.Environments, compareEnvironments)
	if m.Environments == nil {
		m.Environments = []Environment{}
	}
	return writeJSON(path, m)
}
