package catalog

import (
	"encoding/json"
	"testing"
)

const Mi = 1 << 20

// pods builds a kubectl pod list from pod specs written as JSON objects.
func pods(t *testing.T, items ...string) []byte {
	t.Helper()
	out, err := json.Marshal(map[string]any{"items": rawItems(items)})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func rawItems(items []string) []json.RawMessage {
	var out []json.RawMessage
	for _, i := range items {
		out = append(out, json.RawMessage(i))
	}
	return out
}

func all(string, string) bool { return true }

func requested(t *testing.T, match func(string, string) bool, items ...string) float64 {
	t.Helper()
	got, err := RequestedMemory(pods(t, items...), match)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func pod(name, spec string) string {
	return `{"metadata":{"namespace":"x","name":"` + name + `"},"status":{"phase":"Running"},"spec":` + spec + `}`
}

func TestRequestedMemoryCountsInitContainersAsTheSchedulerDoes(t *testing.T) {
	// A sidecar keeps running beside the later init container and the containers.
	got := requested(t, all, pod("a", `{"initContainers":[
		{"name":"s","restartPolicy":"Always","resources":{"requests":{"memory":"50Mi"}}},
		{"name":"i","resources":{"requests":{"memory":"200Mi"}}}],
		"containers":[{"name":"a","resources":{"requests":{"memory":"100Mi"}}}]}`))
	if got != 250 {
		t.Errorf("got %v MiB, want 250", got)
	}
}

func TestRequestedMemorySumsMatchingLivePods(t *testing.T) {
	spec := `{"containers":[{"name":"a","resources":{"requests":{"memory":"64Mi"}}}]}`
	// Two replicas, or a DaemonSet on two nodes, are two pods.
	if got := requested(t, all, pod("a-1", spec), pod("a-2", spec)); got != 128 {
		t.Errorf("two pods = %v MiB, want 128", got)
	}
	// An operator's pod is in the list like any other.
	if got := requested(t, all, pod("vlsingle-0", spec)); got != 64 {
		t.Errorf("one pod = %v MiB, want 64", got)
	}
	if got := requested(t, func(_, name string) bool { return name == "a-1" }, pod("a-1", spec), pod("a-2", spec)); got != 64 {
		t.Errorf("match = %v MiB, want 64", got)
	}
}

func TestRequestedMemorySkipsFinishedAndJobPods(t *testing.T) {
	spec := `"spec":{"containers":[{"name":"a","resources":{"requests":{"memory":"64Mi"}}}]}`
	meta := `"metadata":{"namespace":"x","name":"p"`
	got := requested(t, all,
		`{`+meta+`},"status":{"phase":"Succeeded"},`+spec+`}`,
		`{`+meta+`},"status":{"phase":"Failed"},`+spec+`}`,
		`{`+meta+`,"ownerReferences":[{"kind":"Job"}]},"status":{"phase":"Running"},`+spec+`}`,
		`{`+meta+`,"ownerReferences":[{"kind":"ReplicaSet"}]},"status":{"phase":"Pending"},`+spec+`}`,
	)
	if got != 64 {
		t.Errorf("got %v MiB, want 64 for the pod that runs or waits to run", got)
	}
}

func TestRequestedMemoryFractionOfMiB(t *testing.T) {
	got := requested(t, all, pod("a", `{"containers":[{"name":"a","resources":{"requests":{"memory":"1Mi"}}},{"name":"b","resources":{"requests":{"memory":"1"}}}]}`))
	if got <= 1 || got >= 2 {
		t.Errorf("got %v MiB, want a value between 1 and 2 for the caller to round up", got)
	}
}

func TestRequestedMemoryRejectsBadQuantity(t *testing.T) {
	_, err := RequestedMemory(pods(t, pod("a", `{"containers":[{"name":"a","resources":{"requests":{"memory":"lots"}}}]}`)), all)
	if err == nil {
		t.Error("want an error for an unparsable quantity")
	}
}
