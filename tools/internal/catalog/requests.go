package catalog

import (
	"encoding/json"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/resource"
)

// RequestedMemory returns the MiB of memory that the pods in podList, the
// JSON of a kubectl pod list, request for the scheduler. Each pod counts at
// its spec as the API server holds it, so pods that operators create and a
// DaemonSet's pod on every node are in. Pods that match rejects, finished
// pods and pods that a Job owns are not.
func RequestedMemory(podList []byte, match func(ns, name string) bool) (float64, error) {
	var list struct {
		Items []struct {
			Metadata struct {
				Namespace, Name string
				OwnerReferences []struct{ Kind string }
			}
			Spec   map[string]any
			Status struct{ Phase string }
		}
	}
	if err := json.Unmarshal(podList, &list); err != nil {
		return 0, err
	}
	var total int64
	for _, p := range list.Items {
		if !match(p.Metadata.Namespace, p.Metadata.Name) || p.Status.Phase == "Succeeded" || p.Status.Phase == "Failed" ||
			slices.ContainsFunc(p.Metadata.OwnerReferences, func(o struct{ Kind string }) bool { return o.Kind == "Job" }) {
			continue
		}
		n, err := podMemoryRequest(p.Spec)
		if err != nil {
			return 0, fmt.Errorf("pod %s/%s: %w", p.Metadata.Namespace, p.Metadata.Name, err)
		}
		total += n
	}
	return float64(total) / (1 << 20), nil
}

// podMemoryRequest returns the memory a pod needs to be scheduled: the larger
// of what its init containers need while they run and what its containers
// need together. A sidecar (an init container with restartPolicy Always)
// keeps running, so it adds to both the later init containers and the
// containers.
func podMemoryRequest(pod map[string]any) (int64, error) {
	var sidecars, initPeak int64
	inits, _ := pod["initContainers"].([]any)
	for _, v := range inits {
		c, _ := v.(map[string]any)
		n, err := containerMemoryRequest(c)
		if err != nil {
			return 0, err
		}
		if c["restartPolicy"] == "Always" {
			sidecars += n
			initPeak = max(initPeak, sidecars)
		} else {
			initPeak = max(initPeak, sidecars+n)
		}
	}
	running := sidecars
	list, _ := pod["containers"].([]any)
	for _, v := range list {
		c, _ := v.(map[string]any)
		n, err := containerMemoryRequest(c)
		if err != nil {
			return 0, err
		}
		running += n
	}
	return max(initPeak, running), nil
}

// containerMemoryRequest returns the bytes in a container's memory request,
// or in its limit when it sets no request, as the API server defaults it.
func containerMemoryRequest(c map[string]any) (int64, error) {
	res, _ := c["resources"].(map[string]any)
	for _, k := range []string{"requests", "limits"} {
		m, _ := res[k].(map[string]any)
		v, ok := m["memory"]
		if !ok {
			continue
		}
		q, err := resource.ParseQuantity(fmt.Sprint(v))
		if err != nil {
			return 0, fmt.Errorf("container %v: memory %v: %w", c["name"], v, err)
		}
		return q.Value(), nil
	}
	return 0, nil
}
