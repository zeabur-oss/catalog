package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// reason is why a run tests what it tests. The zero reason tests everything.
type reason struct {
	ids   []string // only these
	since string   // smoke: what changed since this git commit
	stale bool     // full: what has no passing run of its current files
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

// choose returns what a run of tier on profile and arch tests for why.
func choose(r *repo, tier string, why reason, profile, arch string) (pick, error) {
	reasons := 0
	for _, set := range []bool{len(why.ids) > 0, why.since != "", why.stale} {
		if set {
			reasons++
		}
	}
	switch {
	case reasons > 1:
		return pick{}, errors.New("name IDs, or use -since or -stale, only one of them")
	case why.since != "" && tier != "smoke":
		return pick{}, errors.New("-since is for the smoke tier")
	case why.stale && tier != "full":
		return pick{}, errors.New("-stale is for the full tier")
	case why.stale:
		return stalePick(r, profile, arch)
	case len(why.ids) > 0:
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
	return filesPick(r, strings.Fields(out))
}

// stalePick is what a full run on profile and arch tests with -stale: each
// component and template that has no run there, whose last run there failed
// or tested other files, and what installs it. A component in the unready
// list of config/e2e.yaml is never measured, so it is not stale on its own.
func stalePick(r *repo, profile, arch string) (pick, error) {
	var cfg struct {
		Unready []string `yaml:"unready"`
	}
	if err := catalog.ReadYAML("config/e2e.yaml", &cfg); err != nil {
		return pick{}, err
	}
	subs, err := profileSubjects(r, profile, pick{all: true})
	if err != nil {
		return pick{}, err
	}
	p := pick{deps: map[string]bool{}}
	for _, s := range subs {
		path, src := measurementPath(s.id), ""
		if c, ok := r.cat.Get(s.id); ok {
			if slices.Contains(cfg.Unready, s.id) {
				continue
			}
			src = c.Path
		} else {
			if arch != templateArch {
				continue
			}
			t, err := templates.Get(r.templates, s.id)
			if err != nil {
				return pick{}, err
			}
			path, src = templateMeasurementPath(s.id), t.Dir
		}
		digest, err := sourceDigest(src)
		if err != nil {
			return pick{}, err
		}
		if !passed(readMeasurement(path), runEnvironment(profile, arch), digest) {
			p.deps[s.id] = true
		}
	}
	return p, nil
}

// passed reports whether the runs of m in the distribution, profile and
// platform of env on their latest day all passed with digest.
func passed(m measurement, env Environment, digest string) bool {
	latest, ok := "", false
	for _, e := range m.Environments {
		if e.Distribution != env.Distribution || e.Profile != env.Profile || e.Platform != env.Platform {
			continue
		}
		good := e.Result == resultPassed && e.SourceDigest == digest
		switch {
		case e.MeasuredAt > latest:
			latest, ok = e.MeasuredAt, good
		case e.MeasuredAt == latest:
			ok = ok && good
		}
	}
	return ok
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
