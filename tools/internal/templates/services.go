package templates

import (
	"errors"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/resource"
)

// CheckServices checks that each service in template.yaml matches the
// rendered object of the same name: its CPU and memory are the sum of the
// object's containers, and its ports are their named ports. An operator's
// custom resource gives its resources; the operator creates its ports.
func CheckServices(r *Template, manifest []byte) error {
	docs, err := DecodeDocuments(manifest)
	if err != nil {
		return err
	}
	var errs []error
	for _, s := range r.Services {
		fail := func(format string, a ...any) {
			errs = append(errs, fmt.Errorf("service %s: %s", s.Name, fmt.Sprintf(format, a...)))
		}
		var containers []any
		var operand map[string]any
		found := false
		for _, d := range docs {
			if text(object(d["metadata"])["name"]) != s.Name {
				continue
			}
			spec := object(d["spec"])
			switch text(d["kind"]) {
			case "Deployment", "StatefulSet", "DaemonSet":
				containers = array(object(object(spec["template"])["spec"])["containers"])
			case "Cluster", "Dragonfly", "MariaDB":
				operand = object(spec["resources"])
			case "PerconaServerMongoDB":
				operand = object(object(array(spec["replsets"])[0])["resources"])
			default:
				continue
			}
			found = true
		}
		if !found {
			fail("no Deployment, StatefulSet, DaemonSet or database object of the same name")
			continue
		}
		var ports []Port
		got := map[string]map[string]*resource.Quantity{}
		add := func(res map[string]any) {
			for _, level := range []string{"requests", "limits"} {
				for _, key := range []string{"cpu", "memory"} {
					v, ok := object(res[level])[key]
					if !ok {
						continue
					}
					q, err := resource.ParseQuantity(fmt.Sprint(v))
					if err != nil {
						fail("%s.%s: %v", level, key, err)
						continue
					}
					if got[level] == nil {
						got[level] = map[string]*resource.Quantity{}
					}
					if got[level][key] == nil {
						got[level][key] = &resource.Quantity{}
					}
					got[level][key].Add(q)
				}
			}
		}
		if operand != nil {
			add(operand)
		}
		for _, c := range containers {
			add(object(object(c)["resources"]))
			for _, p := range array(object(c)["ports"]) {
				if name := text(object(p)["name"]); name != "" {
					n, _ := object(p)["containerPort"].(int)
					ports = append(ports, Port{Name: name, Port: n})
				}
			}
		}
		for _, level := range []string{"requests", "limits"} {
			for _, key := range []string{"cpu", "memory"} {
				want, err := resource.ParseQuantity(s.Resources[level][key])
				if err != nil {
					fail("resources.%s.%s: %v", level, key, err)
					continue
				}
				if q := got[level][key]; q == nil || q.Cmp(want) != 0 {
					fail("resources.%s.%s is %s; the manifests give %v", level, key, s.Resources[level][key], q)
				}
			}
		}
		if operand == nil {
			cmp := func(a, b Port) int { return a.Port - b.Port }
			want := slices.SortedFunc(slices.Values(s.Ports), cmp)
			slices.SortFunc(ports, cmp)
			if !slices.Equal(want, ports) {
				fail("ports are %v; the containers' named ports are %v", want, ports)
			}
		}
	}
	return errors.Join(errs...)
}
