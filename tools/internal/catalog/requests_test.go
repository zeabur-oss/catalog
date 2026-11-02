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

func TestPodMemoryRequest(t *testing.T) {
	cases := []struct {
		name string
		spec string
		want int64
	}{
		{"containers add up", `{"containers":[
			{"name":"a","resources":{"requests":{"memory":"100Mi"}}},
			{"name":"b","resources":{"requests":{"memory":"0.5Gi"}}}]}`, 612 * Mi},
		{"init larger than containers", `{"initContainers":[{"name":"i","resources":{"requests":{"memory":"300Mi"}}}],
			"containers":[{"name":"a","resources":{"requests":{"memory":"100Mi"}}}]}`, 300 * Mi},
		{"init smaller than containers", `{"initContainers":[{"name":"i","resources":{"requests":{"memory":"10Mi"}}}],
			"containers":[{"name":"a","resources":{"requests":{"memory":"100Mi"}}}]}`, 100 * Mi},
		{"sidecar adds to later init containers and to containers", `{"initContainers":[
			{"name":"s","restartPolicy":"Always","resources":{"requests":{"memory":"50Mi"}}},
			{"name":"i","resources":{"requests":{"memory":"200Mi"}}}],
			"containers":[{"name":"a","resources":{"requests":{"memory":"100Mi"}}}]}`, 250 * Mi},
		{"sidecar adds to containers", `{"initContainers":[
			{"name":"s","restartPolicy":"Always","resources":{"requests":{"memory":"50Mi"}}}],
			"containers":[{"name":"a","resources":{"requests":{"memory":"100Mi"}}}]}`, 150 * Mi},
		{"missing requests count as zero", `{"containers":[
			{"name":"a","resources":{"requests":{"cpu":"10m"}}},{"name":"b"}]}`, 0},
		{"limit stands in for a missing request", `{"containers":[
			{"name":"a","resources":{"limits":{"memory":"128Mi"}}},
			{"name":"b","resources":{"requests":{"memory":"16Mi"},"limits":{"memory":"128Mi"}}}]}`, 144 * Mi},
		{"quantity units", `{"containers":[
			{"name":"a","resources":{"requests":{"memory":"1G"}}},
			{"name":"b","resources":{"requests":{"memory":"128M"}}},
			{"name":"c","resources":{"requests":{"memory":"1048576"}}},
			{"name":"d","resources":{"requests":{"memory":"1Gi"}}}]}`, 1_000_000_000 + 128_000_000 + 1<<20 + 1<<30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var spec map[string]any
			if err := json.Unmarshal([]byte(c.spec), &spec); err != nil {
				t.Fatal(err)
			}
			got, err := podMemoryRequest(spec)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %d bytes, want %d", got, c.want)
			}
		})
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
