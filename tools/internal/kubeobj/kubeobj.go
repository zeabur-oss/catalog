// Package kubeobj reads Kubernetes manifests into objects and asks them typed questions.
package kubeobj

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// Decode returns the objects of a YAML or JSON stream, in order. A List is
// replaced by its items. A document without content, such as a comment, is
// skipped; a document that is not an object is an error.
func Decode(manifest []byte) ([]*unstructured.Unstructured, error) {
	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	var out []*unstructured.Unstructured
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		// This decoder keeps integers as int64, as the objects of the API server have them.
		var m map[string]any
		if err := utiljson.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		if len(m) == 0 {
			continue
		}
		u := &unstructured.Unstructured{Object: m}
		if !u.IsList() {
			out = append(out, u)
			continue
		}
		items, err := u.ToList()
		if err != nil {
			return nil, fmt.Errorf("invalid List: %w", err)
		}
		for i := range items.Items {
			out = append(out, &items.Items[i])
		}
	}
}

// PodTemplate returns the pod of a Pod, or the pod template of a Deployment, StatefulSet,
// DaemonSet, ReplicaSet, Job or CronJob. The bool is false for any other object, and for a
// workload without a template.
func PodTemplate(o *unstructured.Unstructured) (*corev1.PodTemplateSpec, bool, error) {
	var path []string
	switch o.GetKind() {
	case "Pod":
		var pod corev1.Pod
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, &pod); err != nil {
			return nil, false, err
		}
		return &corev1.PodTemplateSpec{ObjectMeta: pod.ObjectMeta, Spec: pod.Spec}, true, nil
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job":
		path = []string{"spec", "template"}
	case "CronJob":
		path = []string{"spec", "jobTemplate", "spec", "template"}
	default:
		return nil, false, nil
	}
	raw, found, err := unstructured.NestedMap(o.Object, path...)
	if err != nil || !found {
		return nil, false, err
	}
	var t corev1.PodTemplateSpec
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &t); err != nil {
		return nil, false, err
	}
	return &t, true, nil
}

// Typed converts the object into out, a Kubernetes type for its kind.
func Typed(o *unstructured.Unstructured, out any) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, out)
}

// Containers returns the init, regular and ephemeral containers of a pod, with the kind of list
// each came from.
func Containers(p *corev1.PodSpec) []Container {
	var out []Container
	for _, c := range p.InitContainers {
		out = append(out, Container{c, "initContainers"})
	}
	for _, c := range p.Containers {
		out = append(out, Container{c, "containers"})
	}
	for _, c := range p.EphemeralContainers {
		out = append(out, Container{corev1.Container(c.EphemeralContainerCommon), "ephemeralContainers"})
	}
	return out
}

// Container is a container and the pod field it is listed in.
type Container struct {
	corev1.Container
	List string
}
