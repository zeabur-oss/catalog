package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// measurement is measurements/<capability>/<upstream>.json. Environments are
// keyed "<profile>-<architecture>-<server version>".
type measurement struct {
	Component    string                 `json:"component,omitempty"`
	Environments map[string]environment `json:"environments"`
}

type environment struct {
	IdleMemoryMiB float64           `json:"idleMemoryMiB"`
	IdleCPUm      float64           `json:"idleCPUm"`
	Versions      map[string]string `json:"versions,omitempty"`
	MeasuredAt    string            `json:"measuredAt"`
}

func e2e(fs *flag.FlagSet, cat *catalog.Catalog) error {
	profile := fs.String("profile", "", "single-node or multi-node; the current cluster must match it")
	write := fs.Bool("write", false, "update measurements/ for passing components")
	previous := fs.String("previous", "", "previous release's index.json, for upgrade tests")
	fs.Parse(os.Args[2:])

	if !slices.Contains(catalog.Profiles, *profile) {
		return fmt.Errorf("unknown -profile %q; use %s", *profile, strings.Join(catalog.Profiles, " or "))
	}
	out, err := catalog.Run(nil, "kubectl", "version", "-o", "json")
	if err != nil {
		return err
	}
	var version struct {
		ServerVersion struct{ GitVersion, Platform string }
	}
	if err := json.Unmarshal([]byte(out), &version); err != nil {
		return err
	}
	_, arch, _ := strings.Cut(version.ServerVersion.Platform, "/") // linux/amd64
	key := *profile + "-" + arch + "-" + version.ServerVersion.GitVersion

	var e2eCfg struct {
		Overrides    map[string]map[string]string            `yaml:"overrides"`
		Replacements map[string]map[string]map[string]string `yaml:"replacements"`
		Unready      []string                                `yaml:"unready"`
	}
	if err := catalog.ReadYAML("config/e2e.yaml", &e2eCfg); err != nil {
		return err
	}
	t := &catalog.Tester{Catalog: cat, Profile: *profile, Overrides: e2eCfg.Overrides,
		Replacements: e2eCfg.Replacements, Unready: map[string]bool{}}
	for _, id := range e2eCfg.Unready {
		if _, ok := cat.Get(id); !ok {
			return fmt.Errorf("config/e2e.yaml: unready: unknown component %q", id)
		}
		t.Unready[id] = true
	}
	if *previous != "" {
		if t.Previous, err = readPrevious(*previous); err != nil {
			return err
		}
	}
	comps, err := selectComponents(cat, fs.Args())
	if err != nil {
		return err
	}

	var results []catalog.Result
	for _, c := range comps {
		if c.Defaults[*profile] == "unsupported" {
			continue
		}
		slog.Info("testing", "component", c.ID, "env", key)
		res := t.Test(c.ID)
		results = append(results, res)
		if res.Err == nil && !res.Unmeasured && *write {
			if err := writeMeasurement(c, key, res); err != nil {
				return err
			}
		}
	}

	fmt.Printf("\n%s:\n", key)
	var errs []error
	for _, r := range results {
		status := fmt.Sprintf("PASS %4.0f MiB %3.0fm CPU", r.IdleMemoryMiB, r.IdleCPUm)
		if r.Unmeasured {
			status = "PASS (installed without credentials; not measured)"
		}
		if r.Err != nil {
			status = "FAIL " + r.Err.Error()
			errs = append(errs, fmt.Errorf("%s: %w", r.ID, r.Err))
		}
		if r.UpgradedFrom != "" {
			status += " (upgraded from " + r.UpgradedFrom + ")"
		}
		fmt.Printf("  %-48s %s\n", r.ID, status)
	}
	return errors.Join(errs...)
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
