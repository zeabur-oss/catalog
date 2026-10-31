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
)

// merge merges the measurements that CI jobs uploaded into measurements/.
func merge(fs *flag.FlagSet, _ *repo) error {
	fs.Parse(os.Args[2:])
	if fs.NArg() != 1 {
		return errors.New("usage: catalog merge <dir>")
	}
	return mergeMeasurements(fs.Arg(0), "measurements")
}

// supersedes reports whether old, an environment already merged, wins over
// env. Every job uploads the whole file it checked out, so an artifact can
// carry stale copies of environments that another job measured; those must
// not undo a newer result or a failure of the same day.
func supersedes(env, old environment) bool {
	if old.MeasuredAt != env.MeasuredAt {
		return old.MeasuredAt > env.MeasuredAt
	}
	return old.Failed && !env.Failed
}

// mergeMeasurements merges each subdirectory of src into dst. A subdirectory
// holds the files of one job, in the layout of measurements/. An environment
// in src replaces the same environment in dst unless dst's is newer, and dst
// keeps the others. An environment equal to dst's before the merge is the
// job's checked-out copy, not a result, so it never replaces another job's:
// a rerun on the same day dates both the same.
func mergeMeasurements(src, dst string) error {
	jobs, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	base := map[string]measurement{}
	for _, job := range jobs {
		if !job.IsDir() {
			continue
		}
		root := filepath.Join(src, job.Name())
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
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
			var in measurement
			if err := json.Unmarshal(data, &in); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			target := filepath.Join(dst, rel)
			out := readMeasurement(target)
			before, seen := base[target]
			if !seen {
				before = readMeasurement(target)
				base[target] = before
			}
			out.Component = cmp.Or(in.Component, out.Component)
			out.Template = cmp.Or(in.Template, out.Template)
			for k, env := range in.Environments {
				if was, ok := before.Environments[k]; ok && reflect.DeepEqual(env, was) {
					continue
				}
				if old, ok := out.Environments[k]; !ok || !supersedes(env, old) {
					out.Environments[k] = env
				}
			}
			return writeJSON(target, out)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
