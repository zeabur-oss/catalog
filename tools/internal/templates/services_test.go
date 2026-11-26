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

func TestCheckServicesReadsTheMySQLServer(t *testing.T) {
	manifest := []byte(`apiVersion: moco.cybozu.com/v1beta2
kind: MySQLCluster
metadata: {name: db}
spec:
  podTemplate:
    spec:
      containers:
        - name: mysqld
          resources:
            requests: {cpu: 100m, memory: 256Mi}
            limits: {cpu: "1", memory: 1Gi}
`)
	r := &templates.Template{ID: "app", Services: []templates.Service{{
		Name:      "db",
		Resources: map[string]map[string]string{"requests": {"cpu": "100m", "memory": "256Mi"}, "limits": {"cpu": "1", "memory": "1Gi"}},
	}}}
	if err := templates.CheckServices(r, manifest); err != nil {
		t.Fatalf("the mysqld container's resources rejected: %v", err)
	}
	r.Services[0].Resources["limits"]["memory"] = "2Gi"
	if err := templates.CheckServices(r, manifest); err == nil {
		t.Fatal("memory limit other than mysqld's accepted")
	}
}
