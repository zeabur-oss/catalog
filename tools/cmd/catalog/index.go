package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/templates"
)

// indexVersion changes only on incompatible changes to index.json.
const indexVersion = 2

type budgetConfig struct {
	SingleNode struct {
		BudgetMiB float64 `yaml:"budgetMiB" json:"budgetMiB"`
	} `yaml:"singleNode" json:"singleNode"`
}

// index writes index.json: every component with its install order,
// measurements and the distributions it was tested on, recommended components per capability, every template with
// its measurements, the budget and the Kubernetes versions.
func index(fs *flag.FlagSet, r *repo) error {
	cat := r.cat
	out := fs.String("out", "index.json", "output file")
	fs.Parse(os.Args[2:])
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	var budget budgetConfig
	if err := catalog.ReadYAML("config/budget.yaml", &budget); err != nil {
		return err
	}
	licenses, err := catalog.ReadLicenses(".")
	if err != nil {
		return err
	}
	type component struct {
		*catalog.Component
		InstallOrder   map[string][]string    `json:"installOrder"`
		TestedOn       []string               `json:"testedOn"`
		Measured       map[string]environment `json:"measured,omitempty"`
		NonOSILicenses []string               `json:"nonOSILicenses,omitempty"`
	}
	capabilities := map[string]map[string][]string{}
	var comps []component
	for _, c := range cat.Components {
		ic := component{Component: c, InstallOrder: map[string][]string{}, NonOSILicenses: licenses.NotOSIApproved(c.Metadata.License)}
		for _, p := range catalog.Profiles {
			if c.Defaults[p] == "unsupported" {
				continue
			}
			order, err := cat.InstallOrder([]string{c.ID}, p)
			if err != nil {
				return err
			}
			for _, o := range order {
				ic.InstallOrder[p] = append(ic.InstallOrder[p], o.ID)
			}
		}
		measured := readMeasurement(measurementPath(c.ID)).Environments
		ic.TestedOn = testedOn(cfg, measured)
		ic.Measured = passed(measured)
		comps = append(comps, ic)
		if capabilities[c.Capability] == nil {
			capabilities[c.Capability] = map[string][]string{}
		}
		for _, p := range catalog.Profiles {
			if c.Defaults[p] == "recommended" {
				capabilities[c.Capability][p] = append(capabilities[c.Capability][p], c.ID)
			}
		}
	}
	return writeJSON(*out, map[string]any{
		"indexVersion": indexVersion,
		"profiles":     catalog.Profiles,
		"kubernetes":   cfg.Versions,
		"budget":       budget,
		"capabilities": capabilities, // capability -> profile -> recommended IDs
		"components":   comps,
		"templates":    indexTemplates(r.templates),
	})
}

// testedOn returns the host profiles, in the order of hostProfiles, with a
// passing measurement of the version that cfg pins. A failed run marks its
// environment failed, so one failed architecture leaves the distribution out
// until it passes again. Its key is "<profile>-<architecture>-<server version>", and
// the server version ends in the pinned release: v1.36.5+k3s1 for k3s,
// v1.35.7+kubesolo-v1.2.3 for KubeSolo v1.2.3.
func testedOn(cfg config, measured map[string]environment) []string {
	out := []string{}
	for _, p := range hostProfiles {
		suffix := "-" + cfg.pinnedVersion(p)
		pass, fail := false, false
		for key, env := range measured {
			if strings.HasPrefix(key, p+"-") && strings.HasSuffix(key, suffix) {
				pass, fail = pass || !env.Failed, fail || env.Failed
			}
		}
		if pass && !fail {
			out = append(out, p)
		}
	}
	return out
}

// indexTemplate is a template in index.json, with its measurements.
type indexTemplate struct {
	*templates.Template
	Measured map[string]environment `json:"measured,omitempty"`
}

// passed returns the environments that are not marked failed.
func passed(envs map[string]environment) map[string]environment {
	maps.DeleteFunc(envs, func(_ string, e environment) bool { return e.Failed })
	return envs
}

func indexTemplates(ts []*templates.Template) []indexTemplate {
	out := make([]indexTemplate, 0, len(ts))
	for _, t := range ts {
		out = append(out, indexTemplate{Template: t, Measured: readMeasurement(templateMeasurementPath(t.ID)).Environments})
	}
	return out
}

// budget sums the idle memory of every component recommended on single-node,
// measured on single-node kind on amd64, and fails above the budget.
func budget(fs *flag.FlagSet, r *repo) error {
	cat := r.cat
	fs.Parse(os.Args[2:])
	var cfg budgetConfig
	if err := catalog.ReadYAML("config/budget.yaml", &cfg); err != nil {
		return err
	}
	var total float64
	var errs []error
	for _, id := range recommended(cat, "single-node") {
		mib, ok := latest(readMeasurement(measurementPath(id)), "single-node-amd64")
		if !ok {
			errs = append(errs, fmt.Errorf("%s: not measured on single-node amd64", id))
			continue
		}
		total += mib
		fmt.Printf("%-48s %5.0f MiB\n", id, mib)
	}
	fmt.Printf("%-48s %5.0f MiB of %.0f\n", "total", total, cfg.SingleNode.BudgetMiB)
	if total > cfg.SingleNode.BudgetMiB {
		errs = append(errs, fmt.Errorf("%.0f MiB exceeds the %.0f MiB budget", total, cfg.SingleNode.BudgetMiB))
	}
	return errors.Join(errs...)
}

// latest returns the idle memory of the newest cluster version measured for
// a prefix ("single-node-amd64" matches "single-node-amd64-v1.36.4").
func latest(m measurement, prefix string) (float64, bool) {
	keys := slices.Sorted(maps.Keys(m.Environments))
	for _, k := range slices.Backward(keys) {
		if strings.HasPrefix(k, prefix+"-") && !m.Environments[k].Failed {
			return m.Environments[k].IdleMemoryMiB, true
		}
	}
	return 0, false
}
