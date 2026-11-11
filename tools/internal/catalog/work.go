package catalog

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

const mib = 1 << 20

// Amounts is how index.json publishes an amount of resources: memory in whole
// MiB and CPU in whole millicores. A missing field is not set. A resource
// added later is a field added here.
type Amounts struct {
	MemoryMiB int64 `json:"memoryMiB,omitempty"`
	CPUMillis int64 `json:"cpuMillis,omitempty"`
}

// Quantities is a component's own amount of resources: Kubernetes quantities
// by resource name, such as memory: 256Mi.
type Quantities map[string]any

// resourceNames are the resources that Quantities may name.
var resourceNames = []string{"cpu", "memory"}

// Amounts is q rounded up to whole MiB and millicores, or nil when q is empty.
func (q Quantities) Amounts() (*Amounts, error) {
	if len(q) == 0 {
		return nil, nil
	}
	var a Amounts
	for _, name := range slices.Sorted(maps.Keys(q)) {
		v, err := resource.ParseQuantity(fmt.Sprint(q[name]))
		if err != nil {
			return nil, fmt.Errorf("%s: %v is not a quantity", name, q[name])
		}
		if v.Sign() <= 0 {
			return nil, fmt.Errorf("%s: %s is not above zero", name, v.String())
		}
		switch name {
		case "memory":
			a.MemoryMiB = (v.Value() + mib - 1) / mib
		case "cpu":
			a.CPUMillis = v.MilliValue()
		default:
			return nil, fmt.Errorf("%s is not a resource; use %s", name, strings.Join(resourceNames, " or "))
		}
	}
	return &a, nil
}

// IndexResources is resources as index.json publishes it.
type IndexResources struct {
	Work []IndexWork `json:"work"`
}

// IndexWork is one kind of work with its amounts.
type IndexWork struct {
	Name     string   `json:"name"`
	Requests *Amounts `json:"requests,omitempty"`
	Limits   *Amounts `json:"limits,omitempty"`
	Use      *Amounts `json:"use,omitempty"`
}

// IndexResources is the resources of each kind of work as index.json
// publishes them.
func (c *Component) IndexResources() (*IndexResources, error) {
	if c.Resources == nil {
		return nil, nil
	}
	out := &IndexResources{}
	for _, w := range c.Resources.Work {
		iw := IndexWork{Name: w.Name}
		for _, f := range []struct {
			name string
			q    Quantities
			to   **Amounts
		}{{"requests", w.Requests, &iw.Requests}, {"limits", w.Limits, &iw.Limits}, {"use", w.Use, &iw.Use}} {
			a, err := f.q.Amounts()
			if err != nil {
				return nil, fmt.Errorf("%s: resources.work %s: %s: %w", c.ID, w.Name, f.name, err)
			}
			*f.to = a
		}
		out.Work = append(out.Work, iw)
	}
	return out, nil
}
