package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// requestsPath holds the memory requests of every component, written by
// catalog requests and read by catalog index. index runs in the release
// build, which has no network to render charts.
const requestsPath = "requests.json"

// memoryRequests maps a component ID and a profile to MiB.
type memoryRequests map[string]map[string]int64

// requests renders every component, release and profile once and writes the
// memory its pods request to requests.json.
func requests(fs *flag.FlagSet, r *repo) error {
	fs.Parse(os.Args[2:])
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	kv := cfg.Versions[len(cfg.Versions)-1]
	got, err := computeRequests(r.cat, func(c *catalog.Component, rel catalog.Release, p string) ([]byte, error) {
		extra, err := contributions(r.cat, c.ID, rel.Name)
		if err != nil {
			return nil, err
		}
		return catalog.Template(c, rel, p, kv, extra...)
	})
	if err != nil {
		return err
	}
	return writeJSON(requestsPath, got)
}

// checkRequests fails unless requests.json matches the manifests that render
// wrote to out.
func checkRequests(cat *catalog.Catalog, out string, versions []string) error {
	kv := versions[len(versions)-1]
	got, err := computeRequests(cat, func(c *catalog.Component, rel catalog.Release, p string) ([]byte, error) {
		return os.ReadFile(filepath.Join(out, kv, p, c.ID, rel.Name+".yaml"))
	})
	if err != nil {
		return err
	}
	if want := readRequests(); !reflect.DeepEqual(got, want) {
		return fmt.Errorf("%s is out of date; run catalog requests", requestsPath)
	}
	return nil
}

// computeRequests sums the memory requests of all releases of each component,
// for each profile the component supports.
func computeRequests(cat *catalog.Catalog, manifest func(*catalog.Component, catalog.Release, string) ([]byte, error)) (memoryRequests, error) {
	out := memoryRequests{}
	var errs []error
	for _, c := range cat.Components {
		for _, p := range catalog.Profiles {
			if c.Defaults[p] == "unsupported" {
				continue
			}
			var bytes int64
			for _, rel := range c.Releases {
				data, err := manifest(c, rel, p)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s/%s %s: %w", c.ID, rel.Name, p, err))
					continue
				}
				n, err := catalog.MemoryRequests(data)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s/%s %s: %w", c.ID, rel.Name, p, err))
					continue
				}
				bytes += n
			}
			if out[c.ID] == nil {
				out[c.ID] = map[string]int64{}
			}
			out[c.ID][p] = catalog.MiB(bytes)
		}
	}
	return out, errors.Join(errs...)
}

func readRequests() memoryRequests {
	var m memoryRequests
	if data, err := os.ReadFile(requestsPath); err == nil {
		json.Unmarshal(data, &m)
	}
	return m
}
