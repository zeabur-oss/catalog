package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

func e2e(fs *flag.FlagSet, r *repo) error {
	tier := fs.String("tier", "full", strings.Join(tiers, ", ")+" (see tier.go)")
	cluster := fs.String("cluster", "kind", "smoke, full: kind creates a cluster for each group or test; current uses the cluster that KUBECONFIG names")
	profile := fs.String("profile", templateProfile, "full: single-node, multi-node (kind), or k3s, rke2 or kubesolo (installed on this host)")
	write := fs.Bool("write", false, "full: update measurements/ for passing tests")
	since := fs.String("since", "", "smoke: only what changed since this git commit")
	keep := fs.Bool("keep", false, "keep the clusters")
	fs.Parse(os.Args[2:])
	if *cluster != "kind" && *cluster != "current" {
		return fmt.Errorf("unknown -cluster %q; use kind or current", *cluster)
	}
	return runTier(r, *tier, tierOptions{profile: *profile, why: reason{ids: fs.Args(), since: *since},
		write: *write, keep: *keep, current: *cluster == "current"})
}

// newTester returns a Tester with the test-only settings of config/e2e.yaml.
func newTester(r *repo, profile string) (*catalog.Tester, error) {
	cfg := r.e2e
	t := &catalog.Tester{Catalog: r.cat, Profile: profile, Overrides: cfg.Overrides,
		Replacements: cfg.Replacements, SkipSmoke: cfg.SkipSmoke, Inputs: cfg.Inputs, Unready: map[string]bool{}}
	for id, inputs := range cfg.Inputs {
		c, ok := r.cat.Get(id)
		if !ok {
			return nil, fmt.Errorf("config/e2e.yaml: inputs: unknown component %q", id)
		}
		for name := range inputs {
			if !slices.ContainsFunc(c.Inputs, func(in catalog.Input) bool { return in.Name == name }) {
				return nil, fmt.Errorf("config/e2e.yaml: inputs: %s has no input %q", id, name)
			}
		}
	}
	for _, id := range cfg.Unready {
		if _, ok := r.cat.Get(id); !ok {
			return nil, fmt.Errorf("config/e2e.yaml: unready: unknown component %q", id)
		}
		t.Unready[id] = true
	}
	return t, t.CheckSkipSmoke()
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
