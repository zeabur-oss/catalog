package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// merge merges the measurements that CI jobs uploaded into measurements/, then
// drops each run whose inputs are no longer the repository's.
func merge(fs *flag.FlagSet, r *repo) error {
	keep := fs.String("keep", "", "measurements of an earlier run to keep where no job measured, in the layout of measurements/")
	fs.Parse(os.Args[2:])
	if fs.NArg() != 1 {
		return errors.New("usage: catalog merge [-keep <dir>] <dir>")
	}
	if err := mergeMeasurements(fs.Arg(0), *keep, "measurements"); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	return dropOutdated(r, cfg, "measurements")
}

// mergeMeasurements puts each environment of keep into dst, then each result
// in src. src holds a subdirectory per job, in the layout of measurements/.
// Every job uploads the whole files it checked out, so an environment equal
// to dst's before the merge is a copy, not a result. Jobs measure different
// environments, so no two results meet.
func mergeMeasurements(src, keep, dst string) error {
	before := map[string]measurement{}
	put := func(rel string, in measurement, results bool) error {
		target := filepath.Join(dst, rel)
		out := readMeasurement(target)
		was, seen := before[rel]
		if !seen {
			was = readMeasurement(target)
			before[rel] = was
		}
		out.Component = cmp.Or(in.Component, out.Component)
		out.Template = cmp.Or(in.Template, out.Template)
		for _, env := range in.Environments {
			if old, ok := was.find(env); results && ok && reflect.DeepEqual(env, old) {
				continue
			}
			out.put(env)
		}
		return writeMeasurementFile(target, out)
	}
	if keep != "" {
		err := walkMeasurements(keep, func(rel string, m measurement) error { return put(rel, m, false) })
		if err != nil {
			return err
		}
	}
	jobs, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if !job.IsDir() {
			continue
		}
		err := walkMeasurements(filepath.Join(src, job.Name()), func(rel string, m measurement) error { return put(rel, m, true) })
		if err != nil {
			return err
		}
	}
	return nil
}

// walkMeasurements calls fn with the path relative to root and the content of
// each measurement file under root.
func walkMeasurements(root string, fn func(rel string, m measurement) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var m measurement
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return fn(rel, m)
	})
}

// dropOutdated removes from dst each run whose inputs differ from the
// repository's, and the file of each component or template that is gone.
func dropOutdated(r *repo, cfg config, dst string) error {
	for _, dir := range []string{"components", "templates"} {
		files, err := filepath.Glob(filepath.Join(dst, dir, "*.json"))
		if err != nil {
			return err
		}
		for _, path := range files {
			id := strings.TrimSuffix(filepath.Base(path), ".json")
			known := r.isComponent(id)
			if dir == "templates" {
				_, err := templates.Get(r.templates, id)
				known = err == nil
			}
			if !known {
				if err := os.Remove(path); err != nil {
					return err
				}
				continue
			}
			m := readMeasurement(path)
			var kept []Environment
			for _, e := range m.Environments {
				profile := e.Profile
				if e.Distribution != kindDistribution {
					profile = e.Distribution
				}
				if !slices.Contains(fullProfiles(), profile) {
					continue
				}
				want, err := inputs(r, cfg, id, profile, e.Platform.Architecture)
				if err != nil {
					return err
				}
				if e.sameInputs(want) {
					kept = append(kept, e)
				}
			}
			m.Environments = kept
			if err := writeMeasurementFile(path, m); err != nil {
				return err
			}
		}
	}
	return nil
}
