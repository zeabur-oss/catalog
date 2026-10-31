package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// measurement is measurements/<capability>/<upstream>.json for a component and
// measurements/templates/<id>.json for a template. Environments are keyed
// "<profile>-<architecture>-<server version>".
type measurement struct {
	Component    string                 `json:"component,omitempty"`
	Template     string                 `json:"template,omitempty"`
	Environments map[string]environment `json:"environments"`
}

type environment struct {
	// Failed marks an environment whose last run failed. It replaces the
	// passing measurement before it, which catalog merge would keep.
	Failed        bool              `json:"failed,omitempty"`
	IdleMemoryMiB float64           `json:"idleMemoryMiB"`
	IdleCPUm      float64           `json:"idleCPUm"`
	Versions      map[string]string `json:"versions,omitempty"`
	MeasuredAt    string            `json:"measuredAt"`
}

func e2e(fs *flag.FlagSet, r *repo) error {
	tier := fs.String("tier", "l2", strings.Join(tiers, ", ")+" (see tier.go)")
	cluster := fs.String("cluster", "kind", "l1, l2: kind creates a cluster for each group or test; current uses the cluster that KUBECONFIG names")
	profile := fs.String("profile", templateProfile, "l2: single-node, multi-node (kind), or k3s, rke2 or kubesolo (installed on this host)")
	write := fs.Bool("write", false, "l2: update measurements/ for passing tests")
	previous := fs.String("previous", "", "l2: previous release's index.json, for upgrade tests")
	since := fs.String("since", "", "l1, l2: without ids, only what changed since this git commit")
	keep := fs.Bool("keep", false, "keep the clusters and the L0 API server")
	fs.Parse(os.Args[2:])
	if *cluster != "kind" && *cluster != "current" {
		return fmt.Errorf("unknown -cluster %q; use kind or current", *cluster)
	}
	return runTier(r, *tier, tierOptions{profile: *profile, since: *since, previous: *previous, write: *write, keep: *keep,
		current: *cluster == "current"}, fs.Args())
}

// environmentKey returns "<profile>-<architecture>-<server version>" from
// the output of kubectl version -o json.
func environmentKey(profile, kubectlVersion string) (string, error) {
	var version struct {
		ServerVersion struct{ GitVersion, Platform string }
	}
	if err := json.Unmarshal([]byte(kubectlVersion), &version); err != nil {
		return "", err
	}
	_, arch, _ := strings.Cut(version.ServerVersion.Platform, "/") // linux/amd64
	return profile + "-" + arch + "-" + version.ServerVersion.GitVersion, nil
}

// newTester reads the test-only settings in config/e2e.yaml.
func newTester(cat *catalog.Catalog, profile string) (*catalog.Tester, error) {
	var cfg struct {
		Overrides    map[string]map[string]string            `yaml:"overrides"`
		Replacements map[string]map[string]map[string]string `yaml:"replacements"`
		Unready      []string                                `yaml:"unready"`
	}
	if err := catalog.ReadYAML("config/e2e.yaml", &cfg); err != nil {
		return nil, err
	}
	t := &catalog.Tester{Catalog: cat, Profile: profile, Overrides: cfg.Overrides,
		Replacements: cfg.Replacements, Unready: map[string]bool{}}
	for _, id := range cfg.Unready {
		if _, ok := cat.Get(id); !ok {
			return nil, fmt.Errorf("config/e2e.yaml: unready: unknown component %q", id)
		}
		t.Unready[id] = true
	}
	return t, nil
}

func readPrevious(path string) (map[string]*catalog.Component, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx struct{ Components []*catalog.Component }
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	out := map[string]*catalog.Component{}
	for _, c := range idx.Components {
		out[c.ID] = c
	}
	return out, nil
}

func measurementPath(id string) string { return filepath.Join("measurements", id+".json") }

func templateMeasurementPath(id string) string {
	return filepath.Join("measurements", "templates", id+".json")
}

func writeMeasurement(c *catalog.Component, key string, res catalog.Result) error {
	m := readMeasurement(measurementPath(c.ID))
	m.Component = c.ID
	versions := map[string]string{}
	for _, r := range c.Releases {
		if r.Version != "" {
			versions[r.Name] = r.Version
		}
	}
	m.Environments[key] = environment{
		IdleMemoryMiB: math.Round(res.IdleMemoryMiB),
		IdleCPUm:      math.Round(res.IdleCPUm),
		Versions:      versions,
		MeasuredAt:    time.Now().UTC().Format(time.DateOnly),
	}
	return writeJSON(measurementPath(c.ID), m)
}

// markFailed records that the test of c failed in environment key. On a host
// profile it also marks every environment of profile on arch at the pinned
// version, and records one when there is none, so a failure before the
// cluster answered (key empty) counts too.
func markFailed(cfg config, profile, arch string, c *catalog.Component, key string) error {
	m := readMeasurement(measurementPath(c.ID))
	m.Component = c.ID
	failed := environment{Failed: true, MeasuredAt: time.Now().UTC().Format(time.DateOnly)}
	if key != "" {
		m.Environments[key] = failed
	}
	if slices.Contains(hostProfiles, profile) {
		// With no measurement yet, a placeholder key carries the failure;
		// testedOn lets a later passing run on the architecture replace it.
		m.Environments[profile+"-"+arch+"-"+cfg.pinnedVersion(profile)] = failed
		for k := range m.Environments {
			if strings.HasPrefix(k, profile+"-"+arch+"-") && strings.HasSuffix(k, "-"+cfg.pinnedVersion(profile)) {
				m.Environments[k] = failed
			}
		}
	}
	return writeJSON(measurementPath(c.ID), m)
}

// writeTemplateMeasurement records the usage of template id in environment
// key. Versions are the tags or digests of the images in its manifest.
func writeTemplateMeasurement(id, key string, usage templates.Usage, images []string) error {
	path := templateMeasurementPath(id)
	m := readMeasurement(path)
	m.Template = id
	m.Environments[key] = environment{
		IdleMemoryMiB: math.Round(usage.IdleMemoryMiB),
		IdleCPUm:      math.Round(usage.IdleCPUm),
		Versions:      imageVersions(images),
		MeasuredAt:    time.Now().UTC().Format(time.DateOnly),
	}
	return writeJSON(path, m)
}

// imageVersions maps each image name to its tag, or to its digest when the
// reference has no tag.
func imageVersions(images []string) map[string]string {
	out := map[string]string{}
	for _, ref := range images {
		name, digest, _ := strings.Cut(ref, "@")
		version := digest
		// A colon after the last slash starts a tag. A colon before it is a registry port.
		if i := strings.LastIndex(name, ":"); i > strings.LastIndex(name, "/") {
			name, version = name[:i], name[i+1:]
		}
		out[name] = version
	}
	return out
}

func readMeasurement(path string) measurement {
	m := measurement{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &m)
	}
	if m.Environments == nil {
		m.Environments = map[string]environment{}
	}
	return m
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
