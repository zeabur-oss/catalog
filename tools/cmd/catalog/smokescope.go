package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// scope is what a smoke run without IDs tests.
type scope struct {
	all   bool            // everything
	deps  map[string]bool // changed subjects: they, and what installs them
	exact map[string]bool // only these
}

func (s scope) selects(id string, order []*catalog.Component) bool {
	return s.all || s.exact[id] || s.deps[id] || slices.ContainsFunc(order, func(c *catalog.Component) bool { return s.deps[c.ID] })
}

// smokeScope decides what a smoke run tests after files changed.
//
//   - config/kubernetes.yaml tests everything.
//   - tools/, go.*, flake.*, another config/ file or a workflow tests the
//     smokeRepresentative list in config/e2e.yaml.
//   - A changed component or template is tested with what installs it.
func smokeScope(r *repo, files []string) (scope, error) {
	s := scope{deps: ownIDs(r, files), exact: map[string]bool{}}
	shared := false
	for _, f := range files {
		switch {
		case f == "config/kubernetes.yaml":
			s.all = true
		case strings.HasPrefix(f, "tools/"), strings.HasPrefix(f, "go."), strings.HasPrefix(f, "flake."),
			strings.HasPrefix(f, "config/"), strings.HasPrefix(f, ".github/workflows/"):
			shared = true
		}
	}
	if s.all || !shared {
		return s, nil
	}
	ids, err := representative(r)
	if err != nil {
		return s, err
	}
	for _, id := range ids {
		s.exact[id] = true
	}
	return s, nil
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

// armSelector narrows sel to the components in the smokeRepresentative list.
// The merge queue repeats those on arm64; templates run on amd64 only, as in
// the full tier.
func armSelector(r *repo, sel func(string, []*catalog.Component) bool) (func(string, []*catalog.Component) bool, error) {
	ids, err := representative(r)
	if err != nil {
		return nil, err
	}
	return func(id string, order []*catalog.Component) bool {
		return strings.Contains(id, "/") && slices.Contains(ids, id) && sel(id, order)
	}, nil
}

// smokeSelector is selector for the smoke tier: a change to a shared file
// selects the representative list instead of everything.
func smokeSelector(r *repo, since string) (func(string, []*catalog.Component) bool, error) {
	if since == "" || strings.Trim(since, "0") == "" {
		return func(string, []*catalog.Component) bool { return true }, nil
	}
	out, err := catalog.Run(nil, "git", "diff", "--name-only", since, "HEAD")
	if err != nil {
		return nil, err
	}
	s, err := smokeScope(r, strings.Fields(out))
	if err != nil {
		return nil, err
	}
	return s.selects, nil
}
