package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
)

// merge merges the measurements that CI jobs uploaded into measurements/.
func merge(fs *flag.FlagSet, _ *repo) error {
	fs.Parse(os.Args[2:])
	if fs.NArg() != 1 {
		return errors.New("usage: catalog merge <dir>")
	}
	return mergeMeasurements(fs.Arg(0), "measurements")
}

// mergeMeasurements merges each subdirectory of src into dst. A subdirectory
// holds the files of one job, in the layout of measurements/. An environment
// in src replaces the same environment in dst, and dst keeps the others.
func mergeMeasurements(src, dst string) error {
	jobs, err := os.ReadDir(src)
	if err != nil {
		return err
	}
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
			out.Component = cmp.Or(in.Component, out.Component)
			out.Template = cmp.Or(in.Template, out.Template)
			maps.Copy(out.Environments, in.Environments)
			return writeJSON(target, out)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
