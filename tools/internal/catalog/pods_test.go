package catalog

import (
	"slices"
	"testing"
)

const objects = `{"items":[
 {"kind":"Pod","metadata":{"namespace":"x","name":"web-7d9-a","ownerReferences":[{"kind":"ReplicaSet","name":"web-7d9","controller":true}]}},
 {"kind":"Pod","metadata":{"namespace":"x","name":"web-7d9-b","ownerReferences":[{"kind":"ReplicaSet","name":"web-7d9","controller":true}]}},
 {"kind":"Pod","metadata":{"namespace":"x","name":"web-7d9-c","ownerReferences":[{"kind":"ReplicaSet","name":"web-7d9","controller":true}]}},
 {"kind":"ReplicaSet","metadata":{"namespace":"x","name":"web-7d9","ownerReferences":[{"kind":"Deployment","name":"web","controller":true}]}},
 {"kind":"Pod","metadata":{"namespace":"x","name":"migrate-1-z","ownerReferences":[{"kind":"Job","name":"migrate-1","controller":true}]}},
 {"kind":"Job","metadata":{"namespace":"x","name":"migrate-1","ownerReferences":[{"kind":"CronJob","name":"migrate","controller":true}]}},
 {"kind":"Pod","metadata":{"namespace":"x","name":"db-0","ownerReferences":[{"kind":"StatefulSet","name":"db","controller":true}]}},
 {"kind":"Pod","metadata":{"namespace":"x","name":"agent-q","ownerReferences":[{"kind":"DaemonSet","name":"agent","controller":true}]}},
 {"kind":"Pod","metadata":{"namespace":"x","name":"loose"}}
]}`

func TestPodsOfOneWorkloadFoldToTheMedian(t *testing.T) {
	workloads, err := Workloads([]byte(objects))
	if err != nil {
		t.Fatal(err)
	}
	got := foldPods(map[string]podSample{
		"x/web-7d9-a": {mem: 40, cpu: 2},
		"x/web-7d9-b": {mem: 100, cpu: 5},
		"x/web-7d9-c": {mem: 60, cpu: 3},
		"x/db-0":      {mem: 200, cpu: 9},
	}, workloads)
	want := []PodUsage{
		{Workload: "web", Kind: "Deployment", MemoryMiB: 60, CPUm: 3},
		{Workload: "db", Kind: "StatefulSet", MemoryMiB: 200, CPUm: 9},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestJobAndReplicaSetPodsFoldToTheirOwner(t *testing.T) {
	workloads, err := Workloads([]byte(objects))
	if err != nil {
		t.Fatal(err)
	}
	for pod, want := range map[string]Workload{
		"x/web-7d9-a":   {"Deployment", "web"},
		"x/migrate-1-z": {"CronJob", "migrate"},
		"x/db-0":        {"StatefulSet", "db"},
		"x/agent-q":     {"DaemonSet", "agent"},
		"x/loose":       {"Pod", "loose"},
	} {
		if workloads[pod] != want {
			t.Errorf("%s = %+v, want %+v", pod, workloads[pod], want)
		}
	}
}

const rendered = `
# Source: chart/templates/web.yaml
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec: {replicas: 3}
---
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db}
spec: {}
---
apiVersion: apps/v1
kind: DaemonSet
metadata: {name: agent}
---
apiVersion: v1
kind: Service
metadata: {name: web}
`

func TestTotalIsPodsTimesRenderedReplicas(t *testing.T) {
	pods := []PodUsage{
		{Workload: "web", Kind: "Deployment", MemoryMiB: 20, CPUm: 1},
		{Workload: "db", Kind: "StatefulSet", MemoryMiB: 100, CPUm: 10},
		{Workload: "agent", Kind: "DaemonSet", MemoryMiB: 5, CPUm: 1},
		// Created by an operator, not in the rendered objects.
		{Workload: "vmsingle", Kind: "Deployment", MemoryMiB: 50, CPUm: 4},
	}
	for nodes, wantMem := range map[int]float64{1: 3*20 + 100 + 5 + 50, 3: 3*20 + 100 + 3*5 + 50} {
		replicas, err := Replicas([]byte(rendered), nodes)
		if err != nil {
			t.Fatal(err)
		}
		if mem, _ := Total(pods, replicas); mem != wantMem {
			t.Errorf("%d nodes: total = %v MiB, want %v", nodes, mem, wantMem)
		}
	}
}
