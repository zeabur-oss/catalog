package main

import (
	"cmp"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/sumdb/dirhash"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// measurement is measurements/components/<capability>/<upstream>.json for a
// component and measurements/templates/<id>.json for a template.
type measurement struct {
	Component    string        `json:"component,omitempty"`
	Template     string        `json:"template,omitempty"`
	Environments []environment `json:"environments"`
}

// Results of a run. A reader treats any other value as not passed.
const (
	resultPassed = "passed"
	resultFailed = "failed"
)

// kindDistribution is the distribution of a kind cluster.
const kindDistribution = "kind"

type platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

func (p platform) String() string { return p.OS + "/" + p.Architecture }

// environment is one run of a component or template test. Distribution,
// Profile, Platform, KubernetesVersion and DistributionVersion identify it.
type environment struct {
	Distribution string `json:"distribution"`
	// Profile is the catalog profile whose values the run installed.
	Profile  string   `json:"profile"`
	Platform platform `json:"platform"`
	// KubernetesVersion is empty when the cluster never answered.
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// DistributionVersion is the release of a distribution installed on the
	// host. kind has none.
	DistributionVersion string `json:"distributionVersion,omitempty"`
	// Result is empty in index.json, which lists passed runs only.
	Result string `json:"result,omitempty"`
	// Pods is the idle usage of one pod of each workload the run started.
	// Totals are not stored: a reader multiplies by the replicas of the
	// rendered objects.
	Pods     []catalog.PodUsage `json:"pods,omitempty"`
	Versions []measuredVersion  `json:"versions,omitempty"`
	// SourceDigest is the Go module hash of the component file, or of the
	// template directory, that the run installed.
	SourceDigest string `json:"sourceDigest,omitempty"`
	MeasuredAt   string `json:"measuredAt"`
}

// measuredVersion is the version of a component's release or of a template's image.
type measuredVersion struct {
	Release string `json:"release,omitempty"`
	Image   string `json:"image,omitempty"`
	Version string `json:"version,omitempty"`
	Digest  string `json:"digest,omitempty"`
}

// same reports whether e and o are runs in the same environment.
func (e environment) same(o environment) bool {
	return e.Distribution == o.Distribution && e.Profile == o.Profile && e.Platform == o.Platform &&
		e.KubernetesVersion == o.KubernetesVersion && e.DistributionVersion == o.DistributionVersion
}

func compareEnvironments(a, b environment) int {
	return cmp.Or(cmp.Compare(a.Distribution, b.Distribution), cmp.Compare(a.Profile, b.Profile),
		cmp.Compare(a.Platform.String(), b.Platform.String()), cmp.Compare(a.KubernetesVersion, b.KubernetesVersion),
		cmp.Compare(a.DistributionVersion, b.DistributionVersion))
}

// find returns the run in the environment of e, if any.
func (m *measurement) find(e environment) (environment, bool) {
	i := slices.IndexFunc(m.Environments, e.same)
	if i < 0 {
		return environment{}, false
	}
	return m.Environments[i], true
}

// put replaces the run in the environment of e, or adds e.
func (m *measurement) put(e environment) {
	if i := slices.IndexFunc(m.Environments, e.same); i >= 0 {
		m.Environments[i] = e
		return
	}
	m.Environments = append(m.Environments, e)
}

// environmentOf returns the environment of a cluster from the output of
// kubectl version -o json. profile is the -profile of the run.
func environmentOf(cfg config, profile, kubectlVersion string) (environment, error) {
	var v struct {
		ServerVersion struct{ GitVersion, Platform string }
	}
	if err := json.Unmarshal([]byte(kubectlVersion), &v); err != nil {
		return environment{}, err
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

// hostEnvironment returns the environment of profile on linux/arch, without
// a Kubernetes version. A host profile runs the single-node values.
func hostEnvironment(cfg config, profile, arch string) environment {
	e := environment{Distribution: kindDistribution, Profile: profile, Platform: platform{"linux", arch}}
	if slices.Contains(hostProfiles, profile) {
		e.Distribution, e.Profile, e.DistributionVersion = profile, templateProfile, cfg.pinnedVersion(profile)
	}
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

func releaseVersions(c *catalog.Component) []measuredVersion {
	var out []measuredVersion
	for _, r := range c.Releases {
		if r.Version != "" {
			out = append(out, measuredVersion{Release: r.Name, Version: r.Version})
		}
	}
	return out
}

func writeMeasurement(c *catalog.Component, env environment, res catalog.Result) error {
	digest, err := sourceDigest(c.Path)
	if err != nil {
		return err
	}
	m := readMeasurement(measurementPath(c.ID))
	m.Component = c.ID
	env.Result = resultPassed
	env.Pods = res.Pods
	env.Versions = releaseVersions(c)
	env.SourceDigest = digest
	env.MeasuredAt = today()
	m.put(env)
	return writeMeasurementFile(measurementPath(c.ID), m)
}

// markFailed records that the test of c failed in env, or before the cluster
// answered when env is nil. On a host profile it also marks every run of the
// pinned release on linux/arch, so a failure before the cluster answered
// counts too. With no such run, it records nothing, because a component with
// no run already shows as untested.
func markFailed(cfg config, profile, arch string, c *catalog.Component, env *environment) error {
	m := readMeasurement(measurementPath(c.ID))
	m.Component = c.ID
	failed := func(e environment) environment {
		return environment{Distribution: e.Distribution, Profile: e.Profile, Platform: e.Platform,
			KubernetesVersion: e.KubernetesVersion, DistributionVersion: e.DistributionVersion,
			Result: resultFailed, Versions: releaseVersions(c), MeasuredAt: today()}
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
// images in its manifest. A template's source is its directory.
func writeTemplateMeasurement(tmpl *templates.Template, env environment, pods []catalog.PodUsage, images []string) error {
	digest, err := sourceDigest(tmpl.Dir)
	if err != nil {
		return err
	}
	id := tmpl.ID
	path := templateMeasurementPath(id)
	m := readMeasurement(path)
	m.Template = id
	env.Result = resultPassed
	env.Pods = pods
	env.Versions = imageVersions(images)
	env.SourceDigest = digest
	env.MeasuredAt = today()
	m.put(env)
	return writeMeasurementFile(path, m)
}

// imageVersions returns the tag and the digest of each image reference.
func imageVersions(images []string) []measuredVersion {
	var out []measuredVersion
	for _, ref := range images {
		name, digest, _ := strings.Cut(ref, "@")
		v := measuredVersion{Image: name, Digest: digest}
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
		m.Environments = []environment{}
	}
	return writeJSON(path, m)
}
