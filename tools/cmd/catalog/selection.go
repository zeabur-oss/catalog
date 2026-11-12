package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// reason is why a run tests what it tests. The zero reason tests everything.
type reason struct {
	ids   []string // only these
	since string   // what changed since this git commit
}

// pick is what a run tests.
type pick struct {
	all   bool            // everything
	deps  map[string]bool // these, and what installs them
	exact map[string]bool // only these
}

func (p pick) has(id string, order []*catalog.Component) bool {
	if p.all || p.exact[id] || p.deps[id] {
		return true
	}
	for _, c := range order {
		if p.deps[c.ID] {
			return true
		}
	}
	return false
}

// choose returns what a run of tier tests for why.
func choose(r *repo, tier string, why reason) (pick, error) {
	if len(why.ids) > 0 && why.since != "" {
		return pick{}, errors.New("name IDs or use -since, not both")
	}
	if len(why.ids) > 0 {
		p := pick{exact: map[string]bool{}}
		for _, id := range why.ids {
			if _, err := installOrder(r, id, templateProfile); err != nil {
				return pick{}, err
			}
			p.exact[id] = true
		}
		return p, nil
	}
	if why.since == "" || strings.Trim(why.since, "0") == "" {
		return pick{all: true}, nil
	}
	out, err := catalog.Run(nil, "git", "diff", "--name-only", why.since, "HEAD")
	if err != nil {
		return pick{}, err
	}
	return filesPick(r, tier, strings.Fields(out))
}

// filesPick is what a run of tier tests after files changed. A changed
// component or template is tested with what installs it.
// config/kubernetes.yaml tests everything. Another shared file (another
// config/ file, tools/, go.*, flake.* or a workflow) tests everything in full
// and the smokeRepresentative list of config/e2e.yaml in smoke.
func filesPick(r *repo, tier string, files []string) (pick, error) {
	p := pick{deps: map[string]bool{}, exact: map[string]bool{}}
	shared := false
	for _, f := range files {
		switch {
		case f == "config/kubernetes.yaml":
			p.all = true
		case strings.HasPrefix(f, "config/"), strings.HasPrefix(f, "tools/"), strings.HasPrefix(f, "go."),
			strings.HasPrefix(f, "flake."), strings.HasPrefix(f, ".github/workflows/"):
			shared = true
		case strings.HasPrefix(f, "templates/"):
			id, _, _ := strings.Cut(strings.TrimPrefix(f, "templates/"), "/")
			p.deps[id] = true
		default:
			for _, c := range r.cat.Components {
				if c.Path == f {
					p.deps[c.ID] = true
				}
			}
		}
	}
	if !shared || p.all {
		return p, nil
	}
	if tier != "smoke" {
		return pick{all: true}, nil
	}
	ids, err := representative(r)
	if err != nil {
		return pick{}, err
	}
	for _, id := range ids {
		p.exact[id] = true
	}
	return p, nil
}

// representative returns the smokeRepresentative list of config/e2e.yaml and
// fails on an ID that is no component or template.
func representative(r *repo) ([]string, error) {
	var cfg struct {
		IDs []string `yaml:"smokeRepresentative"`
	}
	if err := catalog.ReadYAML("config/e2e.yaml", &cfg); err != nil {
		return nil, err
	}
	for _, id := range cfg.IDs {
		if _, err := installOrder(r, id, templateProfile); err != nil {
			return nil, fmt.Errorf("config/e2e.yaml: smokeRepresentative: %w", err)
		}
	}
	return cfg.IDs, nil
}

// splitIDs returns the IDs in list, separated by commas.
func splitIDs(list string) []string {
	var ids []string
	for _, id := range strings.Split(list, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
