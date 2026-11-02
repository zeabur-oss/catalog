package catalog

import (
	"encoding/json"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-helpers/resource"
)

// RequestedMemory returns the MiB of memory that the pods in podList, the
// JSON of a kubectl pod list, request for the scheduler. Each pod counts at
// its spec as the API server holds it, so pods that operators create and a
// DaemonSet's pod on every node are in. Pods that match rejects, finished
// pods and pods that a Job owns are not.
func RequestedMemory(podList []byte, match func(ns, name string) bool) (float64, error) {
	var list corev1.PodList
	if err := json.Unmarshal(podList, &list); err != nil {
		return 0, err
	}
	var total int64
	for _, p := range list.Items {
		if !match(p.Namespace, p.Name) || p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed ||
			slices.ContainsFunc(p.OwnerReferences, func(o metav1.OwnerReference) bool { return o.Kind == "Job" }) {
			continue
		}
		requests := resource.PodRequests(&p, resource.PodResourcesOptions{})
		total += requests.Memory().Value()
	}
	return float64(total) / (1 << 20), nil
}
