// Command catalog checks, renders, tests and packages the catalog.
//
//	catalog validate                    schema, licenses, references, value comments
//	catalog render [ids]                helm template every profile and Kubernetes version, then kubeconform
//	catalog list -tier smoke|full [-ids <ids>|-since <ref>|-stale]  e2e CI matrix as JSON
//	catalog e2e -tier check|smoke|full [-cluster current] [-since <ref>|-stale|ids]  run a test tier (see tier.go)
//	catalog budget                      single-node memory budget
//	catalog api -version <v> [-out out/api]  write the static API: catalog.json, components/<id>.json, schema/
//	catalog schema                      write schema/v1/ from the Go types
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
	"api":        api,
	"schema":     schema,
	"crosscheck": crosscheck,
	"merge":      merge,
	"template":   template,
}

func main() {
	if len(os.Args) < 2 || commands[os.Args[1]] == nil {
		fmt.Fprintln(os.Stderr, "usage: catalog validate|render|list|e2e|budget|api|schema|crosscheck|merge|template [flags] [ids]")
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
	_, repErr := representative(r)
	if err := errors.Join(catalog.Check(".", r.cat), templates.Check(".", r.cat, r.templates), repErr); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

// list prints the e2e CI matrix of a tier. For smoke: one job per group, with
// the IDs it tests. For full: every profile × architecture × component it
// supports, and every template on single-node amd64. Both carry the kind node
// version.
func list(fs *flag.FlagSet, r *repo) error {
	since := fs.String("since", "", "smoke: only what changed since this git commit")
	stale := fs.Bool("stale", false, "full: only what has no passing run of its current files")
	tier := fs.String("tier", "full", "smoke or full")
	only := fs.String("profiles", "", "full: only these profiles, separated by commas")
	idList := fs.String("ids", "", "only these component or template IDs, separated by commas")
	fs.Parse(os.Args[2:])
	profiles := fullProfiles()
	if *only != "" {
		profiles = strings.Split(*only, ",")
		for _, p := range profiles {
			if !slices.Contains(fullProfiles(), p) {
				return fmt.Errorf("unknown profile %q; use %s", p, strings.Join(fullProfiles(), ", "))
			}
		}
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	why := reason{ids: splitIDs(*idList), since: *since, stale: *stale}
	type job struct {
		Profile   string `json:"profile,omitempty"`
		Arch      string `json:"arch,omitempty"`
		Component string `json:"component,omitempty"`
		Template  string `json:"template,omitempty"`
		IDs       string `json:"ids,omitempty"`
	}
	jobs := []job{}
	switch *tier {
	case "smoke":
		selected, err := choose(r, *tier, why, templateProfile, templateArch)
		if err != nil {
			return err
		}
		groups, err := smokeGroups(r, selected)
		if err != nil {
			return err
		}
		for _, g := range groups {
			jobs = append(jobs, job{IDs: strings.Join(g.IDs, " ")})
		}
	case "full":
		for _, p := range profiles {
			for _, arch := range []string{"amd64", "arm64"} {
				selected, err := choose(r, *tier, why, p, arch)
				if err != nil {
					return err
				}
				subs, err := profileSubjects(r, p, selected)
				if err != nil {
					return err
				}
				for _, s := range subs {
					switch {
					case r.isComponent(s.id):
						jobs = append(jobs, job{Profile: p, Arch: arch, Component: s.id})
					case arch == templateArch:
						jobs = append(jobs, job{Profile: p, Arch: arch, Template: s.id})
					}
				}
			}
		}
	default:
		return fmt.Errorf("unknown -tier %q; use smoke or full", *tier)
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"include": jobs, "version": cfg.E2E.Version,
	})
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
