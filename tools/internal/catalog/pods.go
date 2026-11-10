package catalog

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// PodUsage is the idle usage of one pod of a workload: the median over the
// workload's pods at the measurement time. Replicas and requests are not
// measured; they come from the rendered objects.
type PodUsage struct {
	Workload  string  `json:"workload"`
	Kind      string  `json:"kind"`
	MemoryMiB float64 `json:"memoryMiB"`
	CPUm      float64 `json:"cpum"`
}

// Workload names the controller that a pod belongs to.
type Workload struct{ Kind, Name string }

type podSample struct{ mem, cpu float64 }

// Workloads maps "namespace/name" of every pod in list, the JSON of kubectl
// get pods,replicasets,jobs, to its top workload: a ReplicaSet or Job
// resolves to its owner. A pod without a controller is its own workload.
func Workloads(list []byte) (map[string]Workload, error) {
	var l struct {
		Items []struct {
			Kind     string
			Metadata struct {
				Namespace, Name string
				OwnerReferences []struct {
					Kind, Name string
					Controller bool
				}
			}
		}
	}
	if err := json.Unmarshal(list, &l); err != nil {
		return nil, err
	}
	owners := map[string]Workload{}
	for _, o := range l.Items {
		for _, ref := range o.Metadata.OwnerReferences {
			if ref.Controller {
				owners[o.Kind+" "+o.Metadata.Namespace+"/"+o.Metadata.Name] = Workload{ref.Kind, ref.Name}
			}
		}
	}
	out := map[string]Workload{}
	for _, o := range l.Items {
		if o.Kind != "Pod" {
			continue
		}
		ns := o.Metadata.Namespace
		key := ns + "/" + o.Metadata.Name
		w, ok := owners["Pod "+key]
		if !ok {
			out[key] = Workload{"Pod", o.Metadata.Name}
			continue
		}
		for w.Kind == "ReplicaSet" || w.Kind == "Job" {
			up, ok := owners[w.Kind+" "+ns+"/"+w.Name]
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
		out = append(out, PodUsage{Workload: w.Name, Kind: w.Kind, MemoryMiB: math.Round(mem), CPUm: math.Round(cpu)})
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
	out := map[string]int{}
	dec := yaml.NewDecoder(bytes.NewReader(manifest))
	for {
		var obj struct {
			Kind     string
			Metadata struct{ Name string }
			Spec     struct{ Replicas *int }
		}
		if err := dec.Decode(&obj); err == io.EOF {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		key := obj.Kind + "/" + obj.Metadata.Name
		switch obj.Kind {
		case "Deployment", "StatefulSet":
			out[key] = 1
			if obj.Spec.Replicas != nil {
				out[key] = *obj.Spec.Replicas
			}
		case "DaemonSet":
			out[key] = nodes
		}
	}
}

// Total is the idle memory (MiB) and CPU (millicores) of all pods: each row
// times its replicas. A workload without an entry in replicas runs one pod.
func Total(pods []PodUsage, replicas map[string]int) (mem, cpu float64) {
	for _, p := range pods {
		n, ok := replicas[p.Kind+"/"+p.Workload]
		if !ok {
			n = 1
		}
		mem += p.MemoryMiB * float64(n)
		cpu += p.CPUm * float64(n)
	}
	return mem, cpu
}
