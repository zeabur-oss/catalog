package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

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

// newTester reads the test-only settings in config/e2e.yaml.
func newTester(cat *catalog.Catalog, profile string) (*catalog.Tester, error) {
	var cfg struct {
		Overrides    map[string]map[string]string            `yaml:"overrides"`
		Replacements map[string]map[string]map[string]string `yaml:"replacements"`
		SkipSmoke    map[string]map[string]string            `yaml:"skipSmoke"`
		Unready      []string                                `yaml:"unready"`
	}
	if err := catalog.ReadYAML("config/e2e.yaml", &cfg); err != nil {
		return nil, err
	}
	t := &catalog.Tester{Catalog: cat, Profile: profile, Overrides: cfg.Overrides,
		Replacements: cfg.Replacements, SkipSmoke: cfg.SkipSmoke, Unready: map[string]bool{}}
	for _, id := range cfg.Unready {
		if _, ok := cat.Get(id); !ok {
			return nil, fmt.Errorf("config/e2e.yaml: unready: unknown component %q", id)
		}
		t.Unready[id] = true
	}
	return t, t.CheckSkipSmoke()
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
