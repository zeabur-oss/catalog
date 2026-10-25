package templates_test

import (
	"strings"
	"testing"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

func TestCheckServicesComparesTheManifests(t *testing.T) {
	sidecar := strings.Replace(safeManifest, "      containers:\n", `      containers:
        - name: sidecar
          ports: [{name: http, containerPort: 8080}]
          resources:
            requests: {cpu: 5m, memory: 8Mi}
            limits: {cpu: 1, memory: 32Mi}
`, 1)
	service := func(cpu string, ports ...templates.Port) *templates.Template {
		return &templates.Template{ID: "app", Services: []templates.Service{{
			Name:      "app",
			Ports:     ports,
			Resources: map[string]map[string]string{"requests": {"cpu": "15m", "memory": "24Mi"}, "limits": {"cpu": cpu, "memory": "64Mi"}},
		}}}
	}
	if err := templates.CheckServices(service("1100m", templates.Port{Name: "http", Port: 8080}), []byte(sidecar)); err != nil {
		t.Fatalf("sum of the containers rejected: %v", err)
	}
	if err := templates.CheckServices(service("1", templates.Port{Name: "http", Port: 8080}), []byte(sidecar)); err == nil {
		t.Fatal("CPU limit below the sum of the containers accepted")
	}
	if err := templates.CheckServices(service("1100m"), []byte(sidecar)); err == nil {
		t.Fatal("missing port accepted")
	}
	r := service("1100m")
	r.Services[0].Name = "web"
	if err := templates.CheckServices(r, []byte(sidecar)); err == nil {
		t.Fatal("service without an object accepted")
	}
}
