package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"k8s.io/apimachinery/pkg/util/version"
)

// indexVersion changes only on incompatible changes to index.json.
const indexVersion = 3

type budgetConfig struct {
	SingleNode struct {
		BudgetMiB float64 `yaml:"budgetMiB" json:"budgetMiB"`
	} `yaml:"singleNode" json:"singleNode"`
}

// index writes index.json: every component with its install order,
// measurements and the distributions it was tested on, and the Kubernetes versions.
func index(fs *flag.FlagSet, r *repo) error {
	cat := r.cat
	out := fs.String("out", "index.json", "output file")
	fs.Parse(os.Args[2:])
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	type component struct {
		*catalog.Component
		Resources    *catalog.IndexResources `json:"resources,omitempty"`
		InstallOrder map[string][]string     `json:"installOrder"`
		TestedOn     []tested                `json:"testedOn"`
		Measured     []environment           `json:"measured,omitempty"`
	}
	var comps []component
	for _, c := range cat.Components {
		res, err := c.IndexResources()
		if err != nil {
			return err
		}
		ic := component{Component: c, Resources: res, InstallOrder: map[string][]string{}}
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
	}
	return writeJSON(*out, map[string]any{
		"indexVersion": indexVersion,
		"profiles":     catalog.Profiles,
		"kubernetes":   cfg.Versions,
		"components":   comps,
	})
}

// tested is a distribution in testedOn with the platforms it passed on.
type tested struct {
	Distribution string   `json:"distribution"`
	Platforms    []string `json:"platforms"`
}

// testedOn returns the host profiles, in the order of hostProfiles, with the
// platforms where the newest run of the pinned release passed. A failure
// stands until a later run passes on the same platform, at any Kubernetes
// version. A pass on the same day as a failure wins.
func testedOn(cfg config, envs []environment) []tested {
	out := []tested{}
	for _, p := range hostProfiles {
		passAt, failAt := map[string]string{}, map[string]string{}
		for _, e := range envs {
			if e.Distribution != p || e.DistributionVersion != cfg.pinnedVersion(p) {
				continue
			}
			at := failAt
			if e.Result == resultPassed {
				at = passAt
			}
			if pl := e.Platform.String(); e.MeasuredAt > at[pl] {
				at[pl] = e.MeasuredAt
			}
		}
		var platforms []string
		for pl, pass := range passAt {
			if fail, ok := failAt[pl]; !ok || pass >= fail {
				platforms = append(platforms, pl)
			}
		}
		if len(platforms) > 0 {
			slices.Sort(platforms)
			out = append(out, tested{Distribution: p, Platforms: platforms})
		}
	}
	return out
}

// passed returns the runs that passed with a measurement, without their result.
func passed(envs []environment) []environment {
	var out []environment
	for _, e := range envs {
		if e.Result == resultPassed && e.Idle != nil {
			e.Result = ""
			out = append(out, e)
		}
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
	var total int64
	var errs []error
	for _, id := range recommended(cat, "single-node") {
		mib, ok := latest(readMeasurement(measurementPath(id)))
		if !ok {
			errs = append(errs, fmt.Errorf("%s: not measured on single-node amd64", id))
			continue
		}
		total += mib
		fmt.Printf("%-48s %5d MiB\n", id, mib)
	}
	fmt.Printf("%-48s %5d MiB of %.0f\n", "total", total, cfg.SingleNode.BudgetMiB)
	if float64(total) > cfg.SingleNode.BudgetMiB {
		errs = append(errs, fmt.Errorf("%d MiB exceeds the %.0f MiB budget", total, cfg.SingleNode.BudgetMiB))
	}
	return errors.Join(errs...)
}

// latest returns the idle memory of the newest Kubernetes version that
// passed on single-node kind on linux/amd64.
func latest(m measurement) (int64, bool) {
	var best *environment
	for _, e := range m.Environments {
		if e.Distribution != kindDistribution || e.Profile != "single-node" || e.Platform != (platform{"linux", "amd64"}) ||
			e.Result != resultPassed || e.Idle == nil {
			continue
		}
		if best == nil || olderVersion(best.KubernetesVersion, e.KubernetesVersion) {
			best = &e
		}
	}
	if best == nil {
		return 0, false
	}
	return best.Idle.MemoryMiB, true
}

// olderVersion reports whether Kubernetes version a is older than b.
func olderVersion(a, b string) bool {
	va, errA := version.ParseGeneric(a)
	vb, errB := version.ParseGeneric(b)
	if errA != nil || errB != nil {
		return a < b
	}
	return va.LessThan(vb)
}
