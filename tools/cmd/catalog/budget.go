package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/version"
)

type budgetConfig struct {
	SingleNode struct {
		BudgetMiB float64 `yaml:"budgetMiB" json:"budgetMiB"`
	} `yaml:"singleNode" json:"singleNode"`
}

// budget sums the idle memory of every component recommended on single-node,
// measured on single-node kind on amd64, and fails above the budget. A
// component's memory is its measured pods times the replicas that its
// rendered objects set on one node.
func budget(fs *flag.FlagSet, r *repo) error {
	cat := r.cat
	fs.Parse(os.Args[2:])
	var cfg budgetConfig
	if err := catalog.ReadYAML("config/budget.yaml", &cfg); err != nil {
		return err
	}
	kube, err := loadConfig()
	if err != nil {
		return err
	}
	var total resource.Quantity
	var errs []error
	for _, id := range recommended(cat, "single-node") {
		pods, ok := latest(readMeasurement(measurementPath(id)))
		if !ok {
			errs = append(errs, fmt.Errorf("%s: not measured on single-node amd64", id))
			continue
		}
		c, _ := cat.Get(id)
		replicas, err := renderedReplicas(cat, c, "single-node", kube.Versions[len(kube.Versions)-1], 1)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
			continue
		}
		mem, _ := catalog.Total(pods, replicas)
		total.Add(mem)
		fmt.Printf("%-48s %8s\n", id, mem.String())
	}
	limit := resource.MustParse(fmt.Sprintf("%.0fMi", cfg.SingleNode.BudgetMiB))
	fmt.Printf("%-48s %8s of %s\n", "total", total.String(), limit.String())
	if total.Cmp(limit) > 0 {
		errs = append(errs, fmt.Errorf("%s exceeds the %s budget", total.String(), limit.String()))
	}
	return errors.Join(errs...)
}

// renderedReplicas returns the replicas of every workload in the objects that
// c renders for profile on a cluster of nodes nodes.
func renderedReplicas(cat *catalog.Catalog, c *catalog.Component, profile, kubeVersion string, nodes int) (map[string]int, error) {
	out := map[string]int{}
	for _, rel := range c.Releases {
		extra, err := contributions(cat, c.ID, rel.Name)
		if err != nil {
			return nil, err
		}
		manifest, err := catalog.Template(c, rel, profile, kubeVersion, extra...)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel.Name, err)
		}
		replicas, err := catalog.Replicas(manifest, nodes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel.Name, err)
		}
		maps.Copy(out, replicas)
	}
	return out, nil
}

// latest returns the pods of the newest Kubernetes version that passed on
// single-node kind on linux/amd64.
func latest(m measurement) ([]catalog.PodUsage, bool) {
	var best *Environment
	for _, e := range m.Environments {
		if e.Distribution != kindDistribution || e.Profile != "single-node" || e.Platform != (Platform{"linux", "amd64"}) ||
			e.Result != resultPassed || len(e.Pods) == 0 {
			continue
		}
		if best == nil || olderVersion(best.KubernetesVersion, e.KubernetesVersion) {
			best = &e
		}
	}
	if best == nil {
		return nil, false
	}
	return best.Pods, true
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
