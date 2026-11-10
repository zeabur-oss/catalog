package catalog

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	gen "github.com/invopop/jsonschema"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Quantities is a component's own amount of resources: Kubernetes quantities
// by resource name, such as memory: 256Mi.
type Quantities map[string]any

func (Quantities) JSONSchema() *gen.Schema {
	q := &gen.Schema{AnyOf: []*gen.Schema{{Type: "string", MinLength: new(uint64(1))}, {Type: "number", ExclusiveMinimum: "0"}}}
	p := gen.NewProperties()
	for _, name := range resourceNames {
		p.Set(name, q)
	}
	return &gen.Schema{Type: "object", Properties: p, AdditionalProperties: gen.FalseSchema, MinProperties: new(uint64(1)),
		Description: "Kubernetes quantities by resource name, such as memory: 256Mi. Each is above zero."}
}

// resourceNames are the resources that Quantities may name.
var resourceNames = []string{"cpu", "memory"}

// check fails unless each value of q is a quantity above zero.
func (q Quantities) check() error {
	for _, name := range slices.Sorted(maps.Keys(q)) {
		if !slices.Contains(resourceNames, name) {
			return fmt.Errorf("%s is not a resource; use %s", name, strings.Join(resourceNames, " or "))
		}
		v, err := resource.ParseQuantity(fmt.Sprint(q[name]))
		if err != nil {
			return fmt.Errorf("%s: %v is not a quantity", name, q[name])
		}
		if v.Sign() <= 0 {
			return fmt.Errorf("%s: %s is not above zero", name, v.String())
		}
	}
	return nil
}

// checkResources fails unless every quantity of every kind of work is above zero.
func (c *Component) checkResources() error {
	for _, w := range c.Resources.Work {
		for _, f := range []struct {
			name string
			q    Quantities
		}{{"requests", w.Requests}, {"limits", w.Limits}, {"use", w.Use}} {
			if err := f.q.check(); err != nil {
				return fmt.Errorf("resources.work %s: %s: %w", w.Name, f.name, err)
			}
		}
	}
	return nil
}
