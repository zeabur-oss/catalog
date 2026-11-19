package catalog

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zeabur-oss/catalog/tools/internal/kubeobj"
)

// PodUsage is the idle usage of one pod of a workload: the median over the
// workload's pods at the measurement time. Replicas and requests are not
// measured; they come from the rendered objects.
type PodUsage struct {
	Workload string `json:"workload"`
	Kind     string `json:"kind"`
	// Memory and CPU are Kubernetes quantities in JSON, such as "22Mi" and "3m".
	Memory resource.Quantity `json:"memory"`
	CPU    resource.Quantity `json:"cpu"`
}

// NewPodUsage is the usage of a pod of workload, with memory in MiB and CPU
// in millicores rounded to whole numbers.
func NewPodUsage(workload, kind string, memoryMiB, cpuMillis float64) PodUsage {
	return PodUsage{Workload: workload, Kind: kind,
		Memory: *resource.NewQuantity(int64(math.Round(memoryMiB))<<20, resource.BinarySI),
		CPU:    *resource.NewMilliQuantity(int64(math.Round(cpuMillis)), resource.DecimalSI)}
}

// Workload names the controller that a pod belongs to.
type Workload struct{ Kind, Name string }

type podSample struct{ mem, cpu float64 }

// Workloads maps "namespace/name" of every pod in list, the JSON of kubectl
// get pods,replicasets,jobs, to its top workload: a ReplicaSet or Job
// resolves to its owner. A pod without a controller is its own workload.
func Workloads(list []byte) (map[string]Workload, error) {
	var l metav1.PartialObjectMetadataList
	if err := json.Unmarshal(list, &l); err != nil {
		return nil, err
	}
	owners := map[string]Workload{}
	for _, o := range l.Items {
		if ref := metav1.GetControllerOf(&o); ref != nil {
			owners[o.Kind+" "+o.Namespace+"/"+o.Name] = Workload{ref.Kind, ref.Name}
		}
	}
	out := map[string]Workload{}
	for _, o := range l.Items {
		if o.Kind != "Pod" {
			continue
		}
		key := o.Namespace + "/" + o.Name
		w, ok := owners["Pod "+key]
		if !ok {
			out[key] = Workload{"Pod", o.Name}
			continue
		}
		for w.Kind == "ReplicaSet" || w.Kind == "Job" {
			up, ok := owners[w.Kind+" "+o.Namespace+"/"+w.Name]
			if !ok {
				break
			}
			w = up
		}
		out[key] = w
	}
	return out, nil
}

// foldPods turns the usage of each pod, keyed "namespace/name", into one row
// per workload with the median of its pods.
func foldPods(usage map[string]podSample, workloads map[string]Workload) []PodUsage {
	type group struct{ mem, cpu []float64 }
	groups := map[Workload]*group{}
	for key, u := range usage {
		w, ok := workloads[key]
		if !ok {
			_, name, _ := strings.Cut(key, "/")
			w = Workload{"Pod", name}
		}
		if groups[w] == nil {
			groups[w] = &group{}
		}
		groups[w].mem = append(groups[w].mem, u.mem)
		groups[w].cpu = append(groups[w].cpu, u.cpu)
	}
	var out []PodUsage
	for w, g := range groups {
		mem, _ := medianSpread(g.mem)
		cpu, _ := medianSpread(g.cpu)
		out = append(out, NewPodUsage(w.Name, w.Kind, mem, cpu))
	}
	slices.SortFunc(out, func(a, b PodUsage) int {
		return strings.Compare(a.Kind+"/"+a.Workload, b.Kind+"/"+b.Workload)
	})
	return out
}

// Replicas returns the number of pods that each workload runs, keyed
// "Kind/name", from the rendered objects. A DaemonSet runs one pod on each of
// nodes. A workload that manifest does not hold, such as one that an operator
// creates, is missing.
func Replicas(manifest []byte, nodes int) (map[string]int, error) {
	objs, err := kubeobj.Decode(manifest)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, o := range objs {
		key := o.GetKind() + "/" + o.GetName()
		switch o.GetKind() {
		case "Deployment":
			var d appsv1.Deployment
			if err := kubeobj.Typed(o, &d); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			out[key] = replicas(d.Spec.Replicas)
		case "StatefulSet":
			var s appsv1.StatefulSet
			if err := kubeobj.Typed(o, &s); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			out[key] = replicas(s.Spec.Replicas)
		case "DaemonSet":
			out[key] = nodes
		}
	}
	return out, nil
}

// replicas is the replica count of a workload, which defaults to one.
func replicas(n *int32) int {
	if n == nil {
		return 1
	}
	return int(*n)
}

// Total is the idle memory and CPU of all pods: each row times its
// replicas. A workload without an entry in replicas runs one pod.
func Total(pods []PodUsage, replicas map[string]int) (mem, cpu resource.Quantity) {
	for _, p := range pods {
		n, ok := replicas[p.Kind+"/"+p.Workload]
		if !ok {
			n = 1
		}
		for range n {
			mem.Add(p.Memory)
			cpu.Add(p.CPU)
		}
	}
	return mem, cpu
}
