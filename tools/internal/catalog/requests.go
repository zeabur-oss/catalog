package catalog

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
	"k8s.io/apimachinery/pkg/api/resource"
)

// MemoryRequests returns the bytes of memory that the long-running pods in
// manifest request, the way the scheduler counts them. A Deployment or
// StatefulSet counts once per replica and a DaemonSet once, for the node it
// runs on. Jobs and CronJobs are not counted, because they do not run all the
// time. Pods that a custom resource makes an operator create are not in the
// manifest and not counted.
func MemoryRequests(manifest []byte) (int64, error) {
	var total int64
	for _, doc := range strings.Split(string(manifest), "\n---") {
		var obj struct {
			Kind     string
			Metadata struct {
				Name        string
				Annotations map[string]string
			}
			Spec map[string]any
		}
		if yaml.Unmarshal([]byte(doc), &obj) != nil || obj.Metadata.Annotations["helm.sh/hook"] != "" {
			continue
		}
		var pod any
		replicas := int64(1)
		switch obj.Kind {
		case "Pod":
			pod = obj.Spec
		case "DaemonSet":
			pod = dig(obj.Spec, "template", "spec")
		case "Deployment", "StatefulSet":
			pod = dig(obj.Spec, "template", "spec")
			if n, ok := obj.Spec["replicas"].(int); ok {
				replicas = int64(n)
			}
		default:
			continue
		}
		p, _ := pod.(map[string]any)
		n, err := podMemoryRequest(p)
		if err != nil {
			return 0, fmt.Errorf("%s/%s: %w", obj.Kind, obj.Metadata.Name, err)
		}
		total += n * replicas
	}
	return total, nil
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

// MiB converts bytes to whole MiB, rounding up.
func MiB(bytes int64) int64 { return (bytes + 1<<20 - 1) >> 20 }
