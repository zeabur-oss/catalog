package templates_test

import (
	"encoding/json"
	"strings"
	"testing"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestFiniteJobsUseCompletionInsteadOfLongRunningProbes(t *testing.T) {
	r := &templates.Template{}
	r.Services = []templates.Service{{Name: "app", SecurityProfile: "restricted"}}
	for _, kind := range []string{"Deployment", "Job", "CronJob"} {
		t.Run(kind, func(t *testing.T) {
			manifest := strings.ReplaceAll(safeManifest, "          readinessProbe: {tcpSocket: {port: 8080}}\n", "")
			manifest = strings.ReplaceAll(manifest, "          livenessProbe: {tcpSocket: {port: 8080}}\n", "")
			docs, err := templates.DecodeDocuments([]byte(manifest))
			if err != nil {
				t.Fatal(err)
			}
			docs[0]["kind"] = kind
			if kind != "Deployment" {
				docs[0]["apiVersion"] = "batch/v1"
				spec := docs[0]["spec"].(map[string]any)
				pod := spec["template"].(map[string]any)["spec"].(map[string]any)
				pod["restartPolicy"] = "Never"
				if kind == "CronJob" {
					docs[0]["spec"] = map[string]any{"schedule": "0 0 * * *", "jobTemplate": map[string]any{"spec": spec}}
				}
			}
			encode := func() []byte {
				t.Helper()
				var parts []string
				for _, doc := range docs {
					b, e := json.Marshal(doc)
					if e != nil {
						t.Fatal(e)
					}
					parts = append(parts, string(b))
				}
				return []byte(strings.Join(parts, "\n---\n"))
			}
			err = templates.Safety(r, encode(), "test")
			if kind == "Deployment" {
				if err == nil {
					t.Fatal("long-running Deployment without probes accepted")
				}
				return
			}
			if err != nil {
				t.Fatalf("finite %s without probes rejected: %v", kind, err)
			}
			spec := docs[0]["spec"].(map[string]any)
			if kind == "CronJob" {
				spec = spec["jobTemplate"].(map[string]any)["spec"].(map[string]any)
			}
			pod := spec["template"].(map[string]any)["spec"].(map[string]any)
			container := pod["containers"].([]any)[0].(map[string]any)
			container["securityContext"].(map[string]any)["privileged"] = true
			if templates.Safety(r, encode(), "test") == nil {
				t.Fatal("finite workload bypassed container security")
			}
			delete(container["securityContext"].(map[string]any), "privileged")
			container["image"] = "example.test/app:latest"
			if templates.Safety(r, encode(), "test") == nil {
				t.Fatal("finite workload bypassed digest requirement")
			}
			container["image"] = "example.test/app@sha256:" + strings.Repeat("a", 64)
			delete(container, "resources")
			if templates.Safety(r, encode(), "test") == nil {
				t.Fatal("finite workload bypassed resource requirement")
			}
		})
	}
}
