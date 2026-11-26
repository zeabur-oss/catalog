package templates

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/kubeobj"
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
		var pod *corev1.PodTemplateSpec
		var operand map[string]any
		found := false
		for _, d := range docs {
			if d.GetName() != s.Name {
				continue
			}
			spec := object(d.Object["spec"])
			switch kind := d.GetKind(); {
			case slices.Contains(catalog.WorkloadKinds, kind):
				t, _, err := kubeobj.PodTemplate(d)
				if err != nil {
					fail("invalid pod: %v", err)
					continue
				}
				pod = t
			case kind == "Cluster", kind == "Dragonfly", kind == "MariaDB":
				operand = object(spec["resources"])
			case kind == "PerconaServerMongoDB":
				operand = object(object(array(spec["replsets"])[0])["resources"])
			case kind == "MySQLCluster":
				for _, c := range array(object(object(spec["podTemplate"])["spec"])["containers"]) {
					if text(object(c)["name"]) == "mysqld" {
						operand = object(object(c)["resources"])
					}
				}
			default:
				continue
			}
			found = true
		}
		if !found {
			fail("no %s or database object of the same name", strings.Join(catalog.WorkloadKinds, ", "))
			continue
		}
		var ports []Port
		got := map[string]map[string]*resource.Quantity{}
		add := func(res corev1.ResourceRequirements) {
			for level, list := range map[string]corev1.ResourceList{"requests": res.Requests, "limits": res.Limits} {
				for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
					q, ok := list[name]
					if !ok {
						continue
					}
					key := string(name)
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
			var res corev1.ResourceRequirements
			if err := convert(operand, &res); err != nil {
				fail("resources: %v", err)
			}
			add(res)
		}
		if pod != nil {
			for _, c := range pod.Spec.Containers {
				add(c.Resources)
				for _, p := range c.Ports {
					if p.Name != "" {
						ports = append(ports, Port{Name: p.Name, Port: int(p.ContainerPort)})
					}
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
