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
	keep := fs.String("keep", "", "measurements of an earlier run to keep where no job measured, in the layout of measurements/")
	fs.Parse(os.Args[2:])
	if fs.NArg() != 1 {
		return errors.New("usage: catalog merge [-keep <dir>] <dir>")
	}
	return mergeWithKept(fs.Arg(0), *keep, "measurements")
}

// mergeWithKept is mergeMeasurements, then each environment in keep that
// differs from dst before the merge and that no job measured.
func mergeWithKept(src, keep, dst string) error {
	if keep == "" {
		return mergeMeasurements(src, dst)
	}
	kept := map[string]measurement{}
	before := map[string]measurement{}
	err := filepath.WalkDir(keep, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		rel, err := filepath.Rel(keep, path)
		if err != nil {
			return err
		}
		kept[rel] = readMeasurement(path)
		before[rel] = readMeasurement(filepath.Join(dst, rel))
		return nil
	})
	if err != nil {
		return err
	}
	if err := mergeMeasurements(src, dst); err != nil {
		return err
	}
	for rel, k := range kept {
		target := filepath.Join(dst, rel)
		out := readMeasurement(target)
		out.Component = cmp.Or(out.Component, k.Component)
		out.Template = cmp.Or(out.Template, k.Template)
		prior := before[rel]
		for _, env := range k.Environments {
			was, wasOK := prior.find(env)
			if wasOK && reflect.DeepEqual(env, was) {
				continue
			}
			cur, curOK := out.find(env)
			if !curOK || (wasOK && reflect.DeepEqual(cur, was)) {
				out.put(env)
			}
		}
		if err := writeMeasurementFile(target, out); err != nil {
			return err
		}
	}
	return nil
}

// supersedes reports whether old, an environment already merged, wins over
// env. Every job uploads the whole file it checked out, so an artifact can
// carry stale copies of environments that another job measured; those must
// not undo a newer result or a failure of the same day.
func supersedes(env, old Environment) bool {
	if old.MeasuredAt != env.MeasuredAt {
		return old.MeasuredAt > env.MeasuredAt
	}
	return old.Result == resultFailed && env.Result != resultFailed
}

// mergeMeasurements merges each subdirectory of src into dst. A subdirectory
// holds the files of one job, in the layout of measurements/. An environment
// in src replaces the same environment in dst unless dst's is newer, and dst
// keeps the others. An environment equal to dst's before the merge is the
// job's checked-out copy, not a result, and is skipped. A result always
// replaces dst's environment from before the merge, because a rerun on the
// same day dates both the same. Results of two jobs compare by supersedes.
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
			for _, env := range in.Environments {
				was, ok := before.find(env)
				if ok && reflect.DeepEqual(env, was) {
					continue
				}
				old, ok := out.find(env)
				fromBefore := ok && reflect.DeepEqual(old, was)
				if !ok || fromBefore || !supersedes(env, old) {
					out.put(env)
				}
			}
			return writeMeasurementFile(target, out)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
