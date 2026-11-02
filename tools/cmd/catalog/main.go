// Command catalog checks, renders, tests and packages the catalog.
//
//	catalog validate                    schema, licenses, references, value comments
//	catalog render [ids]                helm template every profile and Kubernetes version, then kubeconform
//	catalog list -tier l1|l2 [-since <ref>]  e2e CI matrix as JSON
//	catalog e2e -tier l0|l1|l2 [-cluster current] [ids]  run a test tier (see tier.go)
//	catalog budget                      single-node memory budget
//	catalog index                       write index.json
//	catalog crosscheck                  write crosscheck.json, the render of every template with its e2e values
//	catalog merge <dir>                 merge measurements uploaded by CI jobs into measurements/
//	catalog template render <id>        expand manifests.yaml
//	catalog template inputs <id>        print the Secrets of generated inputs
//	catalog template lint <id>          render, then safety rules and kubeconform
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// repo is everything under components/ and templates/.
type repo struct {
	cat       *catalog.Catalog
	templates []*templates.Template
}

var commands = map[string]func(*flag.FlagSet, *repo) error{
	"validate":   validate,
	"render":     render,
	"list":       list,
	"e2e":        e2e,
	"budget":     budget,
	"index":      index,
	"crosscheck": crosscheck,
	"merge":      merge,
	"template":   template,
}

func main() {
	if len(os.Args) < 2 || commands[os.Args[1]] == nil {
		fmt.Fprintln(os.Stderr, "usage: catalog validate|render|list|e2e|budget|index|crosscheck|merge|template [flags] [ids]")
		os.Exit(2)
	}
	if dir, err := os.UserCacheDir(); err == nil {
		catalog.CacheDir = filepath.Join(dir, "catalog")
	}
	// Isolate helm from the user's repositories and registry logins.
	helm := filepath.Join(catalog.CacheDir, "helm")
	os.Setenv("HELM_REPOSITORY_CONFIG", filepath.Join(helm, "repositories.yaml"))
	os.Setenv("HELM_REPOSITORY_CACHE", filepath.Join(helm, "repository"))
	os.Setenv("HELM_REGISTRY_CONFIG", filepath.Join(helm, "registry.json"))

	name := os.Args[1]
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	r := &repo{}
	var err error
	r.cat, err = catalog.Load(".")
	if err == nil {
		r.templates, err = templates.Load(".")
	}
	if err == nil {
		err = commands[name](fs, r)
	}
	if err != nil {
		slog.Error("catalog "+name+" failed", "err", err)
		os.Exit(1)
	}
}

func validate(fs *flag.FlagSet, r *repo) error {
	fs.Parse(os.Args[2:])
	if err := errors.Join(catalog.Check(".", r.cat), templates.Check(".", r.cat, r.templates)); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

// list prints the e2e CI matrix of a tier. For l1: one job per group, with
// the IDs it tests. For l2: every profile × architecture × component it
// supports, and every template on single-node amd64. Both carry the kind node
// version.
func list(fs *flag.FlagSet, r *repo) error {
	since := fs.String("since", "", "only what changed since this git commit")
	tier := fs.String("tier", "l2", "l1 or l2")
	only := fs.String("profiles", "", "l2: only these profiles, separated by commas")
	idList := fs.String("ids", "", "l2: only these component or template IDs, separated by commas")
	fs.Parse(os.Args[2:])
	ids, err := onlyIDs(r, *idList)
	if err != nil {
		return err
	}
	profiles := l2Profiles()
	if *only != "" {
		profiles = strings.Split(*only, ",")
		for _, p := range profiles {
			if !slices.Contains(l2Profiles(), p) {
				return fmt.Errorf("unknown profile %q; use %s", p, strings.Join(l2Profiles(), ", "))
			}
		}
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	selected, err := selector(r, *since)
	if err != nil {
		return err
	}
	type job struct {
		Profile   string `json:"profile,omitempty"`
		Arch      string `json:"arch,omitempty"`
		Component string `json:"component,omitempty"`
		Template  string `json:"template,omitempty"`
		IDs       string `json:"ids,omitempty"`
	}
	jobs := []job{}
	switch *tier {
	case "l1":
		groups, err := l1Groups(r, selected)
		if err != nil {
			return err
		}
		for _, g := range groups {
			jobs = append(jobs, job{IDs: strings.Join(g.IDs, " ")})
		}
	case "l2":
		for _, p := range profiles {
			subs, err := profileSubjects(r, p, selected)
			if err != nil {
				return err
			}
			for _, s := range subs {
				if ids != nil && !ids[s.id] {
					continue
				}
				if !strings.Contains(s.id, "/") {
					jobs = append(jobs, job{Profile: p, Arch: "amd64", Template: s.id})
					continue
				}
				for _, arch := range []string{"amd64", "arm64"} {
					jobs = append(jobs, job{Profile: p, Arch: arch, Component: s.id})
				}
			}
		}
	default:
		return fmt.Errorf("unknown -tier %q; use l1 or l2", *tier)
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"include": jobs, "version": cfg.E2E.Version,
	})
}

// onlyIDs returns the set of IDs in list, separated by commas, or nil when
// list is empty. An ID that is no component or template is an error.
func onlyIDs(r *repo, list string) (map[string]bool, error) {
	if list == "" {
		return nil, nil
	}
	known := map[string]bool{}
	for _, c := range r.cat.Components {
		known[c.ID] = true
	}
	for _, t := range r.templates {
		known[t.ID] = true
	}
	ids := map[string]bool{}
	for _, id := range strings.Split(list, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !known[id] {
			return nil, fmt.Errorf("unknown component or template %q", id)
		}
		ids[id] = true
	}
	return ids, nil
}

// shared lists the files every e2e run depends on.
var shared = []string{"config/", "tools/", "go.", "flake.", ".github/workflows/"}

// changed returns the IDs of the components and templates whose files
// changed, or nil when a shared file changed.
func changed(r *repo, files []string) map[string]bool {
	ids := map[string]bool{}
	for _, f := range files {
		for _, s := range shared {
			if strings.HasPrefix(f, s) {
				return nil
			}
		}
		for _, c := range r.cat.Components {
			if c.Path == f {
				ids[c.ID] = true
			}
		}
		if rest, ok := strings.CutPrefix(f, "templates/"); ok {
			id, _, _ := strings.Cut(rest, "/")
			ids[id] = true
		}
	}
	return ids
}

func recommended(cat *catalog.Catalog, profile string) []string {
	var ids []string
	for _, c := range cat.Components {
		if c.Defaults[profile] == "recommended" {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// config is config/kubernetes.yaml.
type config struct {
	Versions []string `yaml:"versions"`
	E2E      struct {
		Version string `yaml:"version"` // kindest/node
	} `yaml:"e2e"`
	K3s      k3sConfig      `yaml:"k3s"`
	RKE2     rke2Config     `yaml:"rke2"`
	Kubesolo kubesoloConfig `yaml:"kubesolo"`
}

func loadConfig() (config, error) {
	var cfg config
	return cfg, catalog.ReadYAML("config/kubernetes.yaml", &cfg)
}

func selectComponents(cat *catalog.Catalog, ids []string) ([]*catalog.Component, error) {
	if len(ids) == 0 {
		return cat.Components, nil
	}
	var out []*catalog.Component
	for _, id := range ids {
		c, ok := cat.Get(id)
		if !ok {
			return nil, fmt.Errorf("unknown component %q", id)
		}
		out = append(out, c)
	}
	return out, nil
}
