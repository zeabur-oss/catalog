package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// reason is what a run was asked to test.
type reason struct {
	ids   []string // only these
	since string   // smoke: what changed since this git commit
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

// choose returns what a run of tier on profile and arch tests for why. Named
// IDs are tested alone. Otherwise smoke tests what changed since why.since, or
// everything without it, and full tests what is stale.
func choose(r *repo, tier string, why reason, profile, arch string) (pick, error) {
	switch {
	case len(why.ids) > 0 && why.since != "":
		return pick{}, errors.New("name IDs or use -since, not both")
	case why.since != "" && tier != "smoke":
		return pick{}, errors.New("-since is for the smoke tier")
	case len(why.ids) > 0:
		p := pick{exact: map[string]bool{}}
		for _, id := range why.ids {
			if _, err := installOrder(r, id, templateProfile); err != nil {
				return pick{}, err
			}
			p.exact[id] = true
		}
		return p, nil
	case tier == "full":
		return stalePick(r, profile, arch)
	}
	if why.since == "" || strings.Trim(why.since, "0") == "" {
		return pick{all: true}, nil
	}
	out, err := catalog.Run(nil, "git", "diff", "--name-only", why.since, "HEAD")
	if err != nil {
		return pick{}, err
	}
	return filesPick(r, strings.Fields(out))
}

// stalePick is what a full run on profile and arch tests without IDs: each
// component and template whose inputs there have no passing run, and what
// installs it. A component in the unready list of config/e2e.yaml is never
// measured, so it is not stale on its own.
func stalePick(r *repo, profile, arch string) (pick, error) {
	var e2e struct {
		Unready []string `yaml:"unready"`
	}
	if err := catalog.ReadYAML("config/e2e.yaml", &e2e); err != nil {
		return pick{}, err
	}
	cfg, err := loadConfig()
	if err != nil {
		return pick{}, err
	}
	subs, err := profileSubjects(r, profile, pick{all: true})
	if err != nil {
		return pick{}, err
	}
	p := pick{deps: map[string]bool{}}
	for _, s := range subs {
		path := measurementPath(s.id)
		if r.isComponent(s.id) {
			if slices.Contains(e2e.Unready, s.id) {
				continue
			}
		} else {
			if arch != templateArch {
				continue
			}
			path = templateMeasurementPath(s.id)
		}
		want, err := inputs(r, cfg, s.id, profile, arch)
		if err != nil {
			return pick{}, err
		}
		if !slices.ContainsFunc(readMeasurement(path).Environments, func(e Environment) bool {
			return e.Result == resultPassed && e.sameInputs(want)
		}) {
			p.deps[s.id] = true
		}
	}
	return p, nil
}

// inputs returns what the repository decides about a run of id on profile
// and linux/arch: the source digest of id and of each component installed
// before it, and the release that config/kubernetes.yaml pins, the
// Kubernetes release of kind or the release of a host distribution.
func inputs(r *repo, cfg config, id, profile, arch string) (Environment, error) {
	values, src := profile, ""
	if slices.Contains(hostProfiles, profile) {
		values = templateProfile
	}
	if c, ok := r.cat.Get(id); ok {
		src = c.Path
	} else {
		t, err := templates.Get(r.templates, id)
		if err != nil {
			return Environment{}, err
		}
		src = t.Dir
	}
	order, err := installOrder(r, id, values)
	if err != nil {
		return Environment{}, err
	}
	e := hostEnvironment(cfg, profile, arch)
	if e.Distribution == kindDistribution {
		e.KubernetesVersion = cfg.E2E.Version
	}
	if e.SourceDigest, err = sourceDigest(src); err != nil {
		return Environment{}, err
	}
	for _, c := range order {
		if c.ID == id {
			continue
		}
		digest, err := sourceDigest(c.Path)
		if err != nil {
			return Environment{}, err
		}
		if e.Dependencies == nil {
			e.Dependencies = map[string]string{}
		}
		e.Dependencies[c.ID] = digest
	}
	return e, nil
}

// filesPick is what a smoke run tests after files changed. A changed
// component or template is tested with what installs it.
// config/kubernetes.yaml tests everything. Another shared file (another
// config/ file, tools/, go.*, flake.* or a workflow) tests the
// smokeRepresentative list of config/e2e.yaml.
func filesPick(r *repo, files []string) (pick, error) {
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
