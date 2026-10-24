package templates_test

import (
	"encoding/json"
	"strings"
	"testing"

	templates "github.com/zeabur-oss/catalog/tools/internal/templates"
)

const maintenanceManifest = `
---
apiVersion: v1
kind: ServiceAccount
metadata: {name: backup, namespace: test}
automountServiceAccountToken: false
---
apiVersion: v1
kind: ServiceAccount
metadata: {name: unused, namespace: test}
automountServiceAccountToken: false
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: backup, namespace: test}
rules:
  - apiGroups: [apps]
    resources: [deployments/scale]
    verbs: [get, patch]
    resourceNames: [app]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata: {name: backup, namespace: test}
roleRef: {apiGroup: rbac.authorization.k8s.io, kind: Role, name: backup}
subjects:
  - {kind: ServiceAccount, name: backup, namespace: test}
---
apiVersion: batch/v1
kind: Job
metadata: {name: backup, namespace: test}
spec:
  template:
    spec:
      serviceAccountName: backup
      automountServiceAccountToken: false
      restartPolicy: Never
      securityContext:
        runAsNonRoot: true
        seccompProfile: {type: RuntimeDefault}
      volumes:
        - name: kube-api
          projected:
            sources:
              - serviceAccountToken: {path: token, expirationSeconds: 600}
              - configMap:
                  name: kube-root-ca.crt
                  items: [{key: ca.crt, path: ca.crt}]
              - downwardAPI:
                  items: [{path: namespace, fieldRef: {fieldPath: metadata.namespace}}]
      containers:
        - name: backup
          image: example.test/backup@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
          volumeMounts: [{name: kube-api, mountPath: /var/run/maintenance, readOnly: true}]
          securityContext:
            allowPrivilegeEscalation: false
            capabilities: {drop: [ALL]}
          resources:
            requests: {cpu: 10m, memory: 16Mi}
            limits: {cpu: 100m, memory: 32Mi}
`

func maintenanceRecord() *templates.Template {
	r := &templates.Template{}
	r.Security.Risks = []map[string]string{{"description": "The backup job holds a short-lived API token scoped to scaling this render's application Deployments."}}
	return r
}

func TestSafetyAcceptsNamespacedScaleOnlyMaintenance(t *testing.T) {
	if err := templates.Safety(maintenanceRecord(), []byte(safeManifest+maintenanceManifest), "test"); err != nil {
		t.Fatal(err)
	}
}

func maintenanceObject(docs []map[string]any, kind string) map[string]any {
	for _, d := range docs {
		if d["kind"] == kind {
			return d
		}
	}
	panic("missing maintenance fixture kind " + kind)
}

func maintenancePod(docs []map[string]any) map[string]any {
	return maintenanceObject(docs, "Job")["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
}

func maintenanceToken(docs []map[string]any) map[string]any {
	return maintenancePod(docs)["volumes"].([]any)[0].(map[string]any)["projected"].(map[string]any)["sources"].([]any)[0].(map[string]any)["serviceAccountToken"].(map[string]any)
}

func maintenanceBytes(t *testing.T, docs []map[string]any) []byte {
	t.Helper()
	var out strings.Builder
	for _, d := range docs {
		b, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		out.Write(b)
		out.WriteString("\n---\n")
	}
	return []byte(out.String())
}

func TestSafetyRejectsMaintenancePrivilegeAndTokenEscapes(t *testing.T) {
	cases := map[string]func([]map[string]any){
		"broad-resource":              func(d []map[string]any) { maintenanceRule(d)["resources"] = []any{"deployments"} },
		"secrets-resource":            func(d []map[string]any) { maintenanceRule(d)["resources"] = []any{"secrets"} },
		"pods-resource":               func(d []map[string]any) { maintenanceRule(d)["resources"] = []any{"pods"} },
		"wildcard-resource":           func(d []map[string]any) { maintenanceRule(d)["resources"] = []any{"*"} },
		"wildcard-group":              func(d []map[string]any) { maintenanceRule(d)["apiGroups"] = []any{"*"} },
		"extra-group":                 func(d []map[string]any) { maintenanceRule(d)["apiGroups"] = []any{"apps", ""} },
		"create-verb":                 func(d []map[string]any) { maintenanceRule(d)["verbs"] = []any{"get", "create"} },
		"wildcard-verb":               func(d []map[string]any) { maintenanceRule(d)["verbs"] = []any{"*"} },
		"empty-verbs":                 func(d []map[string]any) { maintenanceRule(d)["verbs"] = []any{} },
		"missing-verbs":               func(d []map[string]any) { delete(maintenanceRule(d), "verbs") },
		"arbitrary-resource-name":     func(d []map[string]any) { maintenanceRule(d)["resourceNames"] = []any{"unrelated"} },
		"wildcard-resource-name":      func(d []map[string]any) { maintenanceRule(d)["resourceNames"] = []any{"*"} },
		"unrestricted-resource-names": func(d []map[string]any) { delete(maintenanceRule(d), "resourceNames") },
		"empty-resource-names":        func(d []map[string]any) { maintenanceRule(d)["resourceNames"] = []any{} },
		"non-resource-urls":           func(d []map[string]any) { maintenanceRule(d)["nonResourceURLs"] = []any{"/healthz"} },
		"extra-rule-field":            func(d []map[string]any) { maintenanceRule(d)["unknown"] = true },
		"extra-broad-rule": func(d []map[string]any) {
			role := maintenanceObject(d, "Role")
			role["rules"] = append(role["rules"].([]any), map[string]any{"apiGroups": []any{""}, "resources": []any{"secrets"}, "verbs": []any{"get"}})
		},
		"cross-namespace-role": func(d []map[string]any) {
			maintenanceObject(d, "Role")["metadata"].(map[string]any)["namespace"] = "other"
		},
		"implicit-role-namespace": func(d []map[string]any) {
			delete(maintenanceObject(d, "Role")["metadata"].(map[string]any), "namespace")
		},
		"cross-namespace-binding": func(d []map[string]any) {
			maintenanceObject(d, "RoleBinding")["metadata"].(map[string]any)["namespace"] = "other"
		},
		"implicit-binding-namespace": func(d []map[string]any) {
			delete(maintenanceObject(d, "RoleBinding")["metadata"].(map[string]any), "namespace")
		},
		"cross-namespace-subject":    func(d []map[string]any) { maintenanceSubject(d)["namespace"] = "other" },
		"implicit-subject-namespace": func(d []map[string]any) { delete(maintenanceSubject(d), "namespace") },
		"user-subject":               func(d []map[string]any) { maintenanceSubject(d)["kind"] = "User" },
		"group-subject":              func(d []map[string]any) { maintenanceSubject(d)["kind"] = "Group" },
		"undeclared-subject":         func(d []map[string]any) { maintenanceSubject(d)["name"] = "other" },
		"wrong-service-account":      func(d []map[string]any) { maintenancePod(d)["serviceAccountName"] = "other" },
		"undeclared-role": func(d []map[string]any) {
			maintenanceObject(d, "RoleBinding")["roleRef"].(map[string]any)["name"] = "other"
		},
		"cluster-role-reference": func(d []map[string]any) {
			maintenanceObject(d, "RoleBinding")["roleRef"].(map[string]any)["kind"] = "ClusterRole"
		},
		"wrong-role-api-group": func(d []map[string]any) {
			maintenanceObject(d, "RoleBinding")["roleRef"].(map[string]any)["apiGroup"] = "other"
		},
		"no-binding":            func(d []map[string]any) { maintenanceObject(d, "RoleBinding")["kind"] = "ConfigMap" },
		"no-role":               func(d []map[string]any) { maintenanceObject(d, "Role")["kind"] = "ConfigMap" },
		"no-short-lived-expiry": func(d []map[string]any) { delete(maintenanceToken(d), "expirationSeconds") },
		"expiry-too-short":      func(d []map[string]any) { maintenanceToken(d)["expirationSeconds"] = 599 },
		"expiry-too-long":       func(d []map[string]any) { maintenanceToken(d)["expirationSeconds"] = 3601 },
		"string-expiry":         func(d []map[string]any) { maintenanceToken(d)["expirationSeconds"] = "600" },
		"fractional-expiry":     func(d []map[string]any) { maintenanceToken(d)["expirationSeconds"] = 600.5 },
		"external-audience":     func(d []map[string]any) { maintenanceToken(d)["audience"] = "https://example.test" },
		"guessed-api-audience":  func(d []map[string]any) { maintenanceToken(d)["audience"] = "https://kubernetes.default.svc" },
		"token-path-traversal":  func(d []map[string]any) { maintenanceToken(d)["path"] = "../token" },
		"token-path-absolute":   func(d []map[string]any) { maintenanceToken(d)["path"] = "/token" },
		"token-path-directory":  func(d []map[string]any) { maintenanceToken(d)["path"] = "dir/token" },
		"token-path-empty":      func(d []map[string]any) { maintenanceToken(d)["path"] = "" },
		"extra-token-field":     func(d []map[string]any) { maintenanceToken(d)["unknown"] = true },
		"pod-token-automount":   func(d []map[string]any) { maintenancePod(d)["automountServiceAccountToken"] = true },
		"account-token-automount": func(d []map[string]any) {
			maintenanceObject(d, "ServiceAccount")["automountServiceAccountToken"] = true
		},
		"no-projected-token": func(d []map[string]any) { delete(maintenancePod(d), "volumes") },
		"unmounted-token": func(d []map[string]any) {
			delete(maintenancePod(d)["containers"].([]any)[0].(map[string]any), "volumeMounts")
		},
		"legacy-account-alias": func(d []map[string]any) { maintenancePod(d)["serviceAccount"] = "other" },
		"long-running-projected-token": func(d []map[string]any) {
			job := maintenanceObject(d, "Job")
			job["kind"], job["apiVersion"] = "Deployment", "apps/v1"
		},
		"unbound-additional-subject": func(d []map[string]any) {
			binding := maintenanceObject(d, "RoleBinding")
			binding["subjects"] = append(binding["subjects"].([]any), map[string]any{"kind": "ServiceAccount", "name": "unused", "namespace": "test"})
		},
		"transitive-broad-grant": func(d []map[string]any) {
			role := maintenanceObject(d, "Role")
			role["rules"] = append(role["rules"].([]any), map[string]any{"apiGroups": []any{"apps"}, "resources": []any{"deployments"}, "resourceNames": []any{"app"}, "verbs": []any{"patch"}})
		},
		"deployment-borrowing-bound-account": func(d []map[string]any) {
			pod := maintenanceObject(d, "Deployment")["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
			pod["serviceAccountName"] = "backup"
		},
		"deployment-legacy-bound-account": func(d []map[string]any) {
			pod := maintenanceObject(d, "Deployment")["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
			pod["serviceAccount"] = "backup"
		},
		"unused-role-and-binding": func(d []map[string]any) { maintenanceObject(d, "Job")["kind"] = "ConfigMap" },
		"cluster-role":            func(d []map[string]any) { maintenanceObject(d, "Role")["kind"] = "ClusterRole" },
		"cluster-role-binding":    func(d []map[string]any) { maintenanceObject(d, "RoleBinding")["kind"] = "ClusterRoleBinding" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			docs, err := templates.DecodeDocuments([]byte(safeManifest + maintenanceManifest))
			if err != nil {
				t.Fatal(err)
			}
			mutate(docs)
			if err := templates.Safety(maintenanceRecord(), maintenanceBytes(t, docs), "test"); err == nil {
				t.Fatal("unsafe maintenance permissions or credentials accepted")
			}
		})
	}
}

func maintenanceRule(docs []map[string]any) map[string]any {
	return maintenanceObject(docs, "Role")["rules"].([]any)[0].(map[string]any)
}

func maintenanceSubject(docs []map[string]any) map[string]any {
	return maintenanceObject(docs, "RoleBinding")["subjects"].([]any)[0].(map[string]any)
}

func TestSafetyRequiresMaintenanceRiskDisclosure(t *testing.T) {
	r := maintenanceRecord()
	r.Security.Risks = nil
	if err := templates.Safety(r, []byte(safeManifest+maintenanceManifest), "test"); err == nil {
		t.Fatal("undisclosed maintenance API trust accepted")
	}
}

func TestSafetyAcceptsScaleOnlyCronJobAndUpperTokenLifetime(t *testing.T) {
	docs, err := templates.DecodeDocuments([]byte(safeManifest + maintenanceManifest))
	if err != nil {
		t.Fatal(err)
	}
	maintenanceToken(docs)["expirationSeconds"] = 3600
	maintenanceToken(docs)["path"] = "maintenance-api.token"
	maintenanceRule(docs)["verbs"] = []any{"get", "update", "patch"}
	job := maintenanceObject(docs, "Job")
	spec := job["spec"]
	job["kind"] = "CronJob"
	job["spec"] = map[string]any{"schedule": "0 3 * * *", "jobTemplate": map[string]any{"spec": spec}}
	if err := templates.Safety(maintenanceRecord(), maintenanceBytes(t, docs), "test"); err != nil {
		t.Fatal(err)
	}
}

func TestSafetyRejectsAdditionalTransitiveGrantToMaintenanceAccount(t *testing.T) {
	docs, err := templates.DecodeDocuments([]byte(safeManifest + maintenanceManifest))
	if err != nil {
		t.Fatal(err)
	}
	docs = append(docs,
		map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "Role",
			"metadata":   map[string]any{"name": "additional", "namespace": "test"},
			"rules":      []any{map[string]any{"apiGroups": []any{""}, "resources": []any{"secrets"}, "verbs": []any{"get"}}},
		},
		map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata":   map[string]any{"name": "additional", "namespace": "test"},
			"roleRef":    map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": "additional"},
			"subjects":   []any{map[string]any{"kind": "ServiceAccount", "name": "backup", "namespace": "test"}},
		},
	)
	if err := templates.Safety(maintenanceRecord(), maintenanceBytes(t, docs), "test"); err == nil {
		t.Fatal("a second RoleBinding granted broader transitive maintenance authority")
	}
}

func TestSafetyRejectsEachUnauthorizedTokenProjection(t *testing.T) {
	docs, err := templates.DecodeDocuments([]byte(safeManifest + maintenanceManifest))
	if err != nil {
		t.Fatal(err)
	}
	volume := maintenancePod(docs)["volumes"].([]any)[0].(map[string]any)
	projected := volume["projected"].(map[string]any)
	projected["sources"] = append(projected["sources"].([]any), map[string]any{"serviceAccountToken": map[string]any{"path": "second", "expirationSeconds": 86400}})
	if err := templates.Safety(maintenanceRecord(), maintenanceBytes(t, docs), "test"); err == nil {
		t.Fatal("valid maintenance token hid another unrestricted token projection")
	}
}
