package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// render runs helm template for every component, release, profile and
// Kubernetes version, with every contribution applied, and checks the output
// with kubeconform.
func render(fs *flag.FlagSet, r *repo) error {
	cat := r.cat
	out := fs.String("out", "out/render", "output directory")
	fs.Parse(os.Args[2:])
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	comps, err := selectComponents(cat, fs.Args())
	if err != nil {
		return err
	}
	return renderComponents(cat, comps, *out, cfg.Versions)
}

func renderComponents(cat *catalog.Catalog, comps []*catalog.Component, out string, versions []string) error {
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	if err := os.RemoveAll(out + "-schemas"); err != nil {
		return err
	}
	var errs []error
	for _, c := range comps {
		for _, rel := range c.Releases {
			extra, err := contributions(cat, c.ID, rel.Name)
			if err != nil {
				return err
			}
			for _, kv := range versions {
				for _, p := range catalog.Profiles {
					if c.Defaults[p] == "unsupported" {
						continue
					}
					manifest, err := catalog.Template(c, rel, p, kv, extra...)
					if err != nil {
						errs = append(errs, fmt.Errorf("%s/%s %s %s: %w", c.ID, rel.Name, p, kv, err))
						continue
					}
					// urlDigests pin a URL release. Its images are the upstream's choice.
					if kv == versions[0] && rel.Kind() != "manifest" {
						for _, u := range catalog.UnpinnedImages(manifest, rel.Unpinned) {
							errs = append(errs, fmt.Errorf("%s/%s %s: %s needs a sha256 digest (<tag>@sha256:<digest>)", c.ID, rel.Name, p, u))
						}
					}
					// Charts must not generate secrets or certificates: the same
					// values must give the same objects on every install.
					if kv == versions[0] && rel.Kind() == "helm" {
						again, err := catalog.Template(c, rel, p, kv, extra...)
						if err == nil && !bytes.Equal(again, manifest) {
							errs = append(errs, fmt.Errorf("%s/%s %s: output differs between renders; supply generated values as inputs", c.ID, rel.Name, p))
						}
					}
					level := "restricted"
					if l, ok := c.Security["podSecurity"].(string); ok {
						level = l
					}
					// A privileged component's namespaces enforce no Pod Security level.
					if priv, _ := c.Security["privileged"].(bool); priv {
						level = ""
					}
					if err := templates.CheckPodSecurity(manifest, level); err != nil {
						errs = append(errs, fmt.Errorf("%s/%s %s %s: %w", c.ID, rel.Name, p, kv, err))
					}
					f := filepath.Join(out, kv, p, c.ID, rel.Name+".yaml")
					if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
						return err
					}
					if err := os.WriteFile(f, manifest, 0o644); err != nil {
						return err
					}
				}
			}
		}
		slog.Info("rendered", "component", c.ID)
	}
	// Custom resources are checked against the CRDs rendered with them.
	schemas := out + "-schemas"
	if err := catalog.CRDSchemas(out, schemas); err != nil {
		return err
	}
	for _, kv := range versions {
		cmd := exec.Command("kubeconform", "-kubernetes-version", kv, "-strict", "-summary", "-skip", "CustomResourceDefinition",
			"-schema-location", "default",
			"-schema-location", schemas+"/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
			filepath.Join(out, kv))
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			errs = append(errs, fmt.Errorf("kubeconform %s: %w", kv, err))
		}
	}
	return errors.Join(errs...)
}

// contributions returns the values other components add to a release.
func contributions(cat *catalog.Catalog, target, release string) ([]map[string]any, error) {
	var out []map[string]any
	for _, c := range cat.Components {
		for _, ct := range c.Contributes {
			if ct.Target == target && ct.Release == release {
				m, err := catalog.ParseValues(ct.Values)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", c.ID, err)
				}
				out = append(out, m)
			}
		}
	}
	return out, nil
}
