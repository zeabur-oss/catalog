package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
)

// alertTests feeds ZeaburCrashLooping the restart counters of three containers
// in a project namespace and one in a namespace that is not a project.
const alertTests = `evaluation_interval: 30s
rule_files: [rules.yaml]
tests:
  - interval: 1m
    input_series:
      - series: 'kube_namespace_labels{namespace="app", label_zeabur_com_project="p1"}'
        values: "1x20"
      - series: 'kube_pod_container_status_restarts_total{namespace="app", pod="loop", container="c"}'
        values: "0 0 1 1 2 2 3 3 4 4 5 5 6 6 7 7 8 8 9 9 10"
      - series: 'kube_pod_container_status_restarts_total{namespace="app", pod="once", container="c"}'
        values: "0 0 0 0 0 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1"
      - series: 'kube_pod_container_status_restarts_total{namespace="app", pod="steady", container="c"}'
        values: "3x20"
      - series: 'kube_pod_container_status_restarts_total{namespace="kube-system", pod="loop", container="c"}'
        values: "0 0 1 1 2 2 3 3 4 4 5 5 6 6 7 7 8 8 9 9 10"
    alert_rule_test:
      - eval_time: 12m
        alertname: ZeaburCrashLooping
        exp_alerts:
          - exp_labels:
              namespace: app
              pod: loop
              container: c
              severity: warning
              zeabur_event: crash-loop
              zeabur_kind: state
            exp_annotations:
              summary: loop keeps crashing.
              description: Container c of loop in app restarted at least twice in 10 minutes.
`

// vmalert-tool runs the rules of the notifications release, as the cluster
// evaluates them.
func TestCrashLoopingFiresOnRestarts(t *testing.T) {
	tool, err := exec.LookPath("vmalert-tool")
	if err != nil {
		t.Skip("needs the dev shell's vmalert-tool")
	}
	var c catalog.Component
	if err := catalog.ReadYAML("../../../components/victoria-metrics.yaml", &c); err != nil {
		t.Fatal(err)
	}
	rel, ok := c.Release("notifications")
	if !ok {
		t.Fatal("no notifications release")
	}
	var rule struct {
		Spec map[string]any `yaml:"spec"`
	}
	if err := yaml.Unmarshal([]byte(rel.Manifests), &rule); err != nil {
		t.Fatal(err)
	}
	rules, err := yaml.Marshal(rule.Spec)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rules.yaml"), rules, 0o644); err != nil {
		t.Fatal(err)
	}
	tests := filepath.Join(dir, "tests.yaml")
	if err := os.WriteFile(tests, []byte(alertTests), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(tool, "unittest", "--disableAlertgroupLabel", "--files", tests)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
