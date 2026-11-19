package kubeobj

import "testing"

const manifest = `# a comment only
---
apiVersion: v1
kind: ConfigMap
metadata: {name: a}
data:
  script: |
    echo one
    ---
    echo two
---
apiVersion: v1
kind: List
items:
  - {apiVersion: batch/v1, kind: CronJob, metadata: {name: c}, spec: {jobTemplate: {spec: {template: {metadata: {labels: {app: web}}, spec: {containers: [{name: app, image: i}], initContainers: [{name: init, image: j}]}}}}}}
  - {apiVersion: v1, kind: Service, metadata: {name: s}}
`

func TestDecodeKeepsScalarsAndExpandsLists(t *testing.T) {
	objs, err := Decode([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, o := range objs {
		kinds = append(kinds, o.GetKind())
	}
	if got := len(kinds); got != 3 || kinds[0] != "ConfigMap" || kinds[1] != "CronJob" || kinds[2] != "Service" {
		t.Fatalf("kinds = %v", kinds)
	}
}

func TestPodTemplate(t *testing.T) {
	objs, err := Decode([]byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range []int{0, 2} {
		if _, ok, err := PodTemplate(objs[o]); ok || err != nil {
			t.Errorf("%s: ok = %v, err = %v", objs[o].GetKind(), ok, err)
		}
	}
	tmpl, ok, err := PodTemplate(objs[1])
	if err != nil || !ok {
		t.Fatalf("CronJob: ok = %v, err = %v", ok, err)
	}
	if tmpl.Labels["app"] != "web" {
		t.Errorf("labels = %v", tmpl.Labels)
	}
	var got []string
	for _, c := range Containers(&tmpl.Spec) {
		got = append(got, c.List+"/"+c.Name)
	}
	if len(got) != 2 || got[0] != "initContainers/init" || got[1] != "containers/app" {
		t.Errorf("containers = %v", got)
	}
}

func TestDecodeFailsOnBrokenYAML(t *testing.T) {
	if _, err := Decode([]byte("kind: [")); err == nil {
		t.Error("want an error")
	}
}
