package templates_test

import (
	"encoding/json"
	"errors"
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
	cases := map[string]struct {
		rule   templates.Rule
		mutate func([]map[string]any)
	}{
		"broad-resource":              {templates.ErrMaintenanceRoleRules, func(d []map[string]any) { maintenanceRule(d)["resources"] = []any{"deployments"} }},
		"secrets-resource":            {templates.ErrMaintenanceRoleRules, func(d []map[string]any) { maintenanceRule(d)["resources"] = []any{"secrets"} }},
		"pods-resource":               {templates.ErrMaintenanceRoleRules, func(d []map[string]any) { maintenanceRule(d)["resources"] = []any{"pods"} }},
		"wildcard-resource":           {templates.ErrMaintenanceRoleRules, func(d []map[string]any) { maintenanceRule(d)["resources"] = []any{"*"} }},
		"wildcard-group":              {templates.ErrMaintenanceRoleRules, func(d []map[string]any) { maintenanceRule(d)["apiGroups"] = []any{"*"} }},
		"extra-group":                 {templates.ErrMaintenanceRoleRules, func(d []map[string]any) { maintenanceRule(d)["apiGroups"] = []any{"apps", ""} }},
		"create-verb":                 {templates.ErrMaintenanceVerbs, func(d []map[string]any) { maintenanceRule(d)["verbs"] = []any{"get", "create"} }},
		"wildcard-verb":               {templates.ErrMaintenanceVerbs, func(d []map[string]any) { maintenanceRule(d)["verbs"] = []any{"*"} }},
		"empty-verbs":                 {templates.ErrMaintenanceVerbs, func(d []map[string]any) { maintenanceRule(d)["verbs"] = []any{} }},
		"missing-verbs":               {templates.ErrMaintenanceVerbs, func(d []map[string]any) { delete(maintenanceRule(d), "verbs") }},
		"arbitrary-resource-name":     {templates.ErrMaintenanceResourceNames, func(d []map[string]any) { maintenanceRule(d)["resourceNames"] = []any{"unrelated"} }},
		"wildcard-resource-name":      {templates.ErrMaintenanceResourceNames, func(d []map[string]any) { maintenanceRule(d)["resourceNames"] = []any{"*"} }},
		"unrestricted-resource-names": {templates.ErrMaintenanceResourceNames, func(d []map[string]any) { delete(maintenanceRule(d), "resourceNames") }},
		"empty-resource-names":        {templates.ErrMaintenanceResourceNames, func(d []map[string]any) { maintenanceRule(d)["resourceNames"] = []any{} }},
		"non-resource-urls":           {templates.ErrMaintenanceRoleRules, func(d []map[string]any) { maintenanceRule(d)["nonResourceURLs"] = []any{"/healthz"} }},
		"extra-rule-field":            {templates.ErrMaintenanceRoleRules, func(d []map[string]any) { maintenanceRule(d)["unknown"] = true }},
		"extra-broad-rule": {templates.ErrMaintenanceRoleRules, func(d []map[string]any) {
			role := maintenanceObject(d, "Role")
			role["rules"] = append(role["rules"].([]any), map[string]any{"apiGroups": []any{""}, "resources": []any{"secrets"}, "verbs": []any{"get"}})
		}},
		"cross-namespace-role": {templates.ErrMaintenanceRoleScope, func(d []map[string]any) {
			maintenanceObject(d, "Role")["metadata"].(map[string]any)["namespace"] = "other"
		}},
		"implicit-role-namespace": {templates.ErrMaintenanceRoleScope, func(d []map[string]any) {
			delete(maintenanceObject(d, "Role")["metadata"].(map[string]any), "namespace")
		}},
		"cross-namespace-binding": {templates.ErrMaintenanceBindingScope, func(d []map[string]any) {
			maintenanceObject(d, "RoleBinding")["metadata"].(map[string]any)["namespace"] = "other"
		}},
		"implicit-binding-namespace": {templates.ErrMaintenanceBindingScope, func(d []map[string]any) {
			delete(maintenanceObject(d, "RoleBinding")["metadata"].(map[string]any), "namespace")
		}},
		"cross-namespace-subject":    {templates.ErrMaintenanceSubjects, func(d []map[string]any) { maintenanceSubject(d)["namespace"] = "other" }},
		"implicit-subject-namespace": {templates.ErrMaintenanceSubjects, func(d []map[string]any) { delete(maintenanceSubject(d), "namespace") }},
		"user-subject":               {templates.ErrMaintenanceSubjects, func(d []map[string]any) { maintenanceSubject(d)["kind"] = "User" }},
		"group-subject":              {templates.ErrMaintenanceSubjects, func(d []map[string]any) { maintenanceSubject(d)["kind"] = "Group" }},
		"undeclared-subject":         {templates.ErrMaintenanceSubjects, func(d []map[string]any) { maintenanceSubject(d)["name"] = "other" }},
		"wrong-service-account":      {templates.ErrMaintenanceTokenBinding, func(d []map[string]any) { maintenancePod(d)["serviceAccountName"] = "other" }},
		"undeclared-role": {templates.ErrMaintenanceBindingRole, func(d []map[string]any) {
			maintenanceObject(d, "RoleBinding")["roleRef"].(map[string]any)["name"] = "other"
		}},
		"cluster-role-reference": {templates.ErrMaintenanceBindingRole, func(d []map[string]any) {
			maintenanceObject(d, "RoleBinding")["roleRef"].(map[string]any)["kind"] = "ClusterRole"
		}},
		"wrong-role-api-group": {templates.ErrMaintenanceBindingRole, func(d []map[string]any) {
			maintenanceObject(d, "RoleBinding")["roleRef"].(map[string]any)["apiGroup"] = "other"
		}},
		"no-binding":            {templates.ErrMaintenanceTokenBinding, func(d []map[string]any) { maintenanceObject(d, "RoleBinding")["kind"] = "ConfigMap" }},
		"no-role":               {templates.ErrMaintenanceBindingRole, func(d []map[string]any) { maintenanceObject(d, "Role")["kind"] = "ConfigMap" }},
		"no-short-lived-expiry": {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { delete(maintenanceToken(d), "expirationSeconds") }},
		"expiry-too-short":      {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["expirationSeconds"] = 599 }},
		"expiry-too-long":       {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["expirationSeconds"] = 3601 }},
		"string-expiry":         {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["expirationSeconds"] = "600" }},
		"fractional-expiry":     {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["expirationSeconds"] = 600.5 }},
		"external-audience":     {templates.ErrMaintenanceTokenAudience, func(d []map[string]any) { maintenanceToken(d)["audience"] = "https://example.test" }},
		"guessed-api-audience":  {templates.ErrMaintenanceTokenAudience, func(d []map[string]any) { maintenanceToken(d)["audience"] = "https://kubernetes.default.svc" }},
		"token-path-traversal":  {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["path"] = "../token" }},
		"token-path-absolute":   {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["path"] = "/token" }},
		"token-path-directory":  {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["path"] = "dir/token" }},
		"token-path-empty":      {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["path"] = "" }},
		"extra-token-field":     {templates.ErrMaintenanceTokenShape, func(d []map[string]any) { maintenanceToken(d)["unknown"] = true }},
		"pod-token-automount":   {templates.ErrMaintenancePodAutomount, func(d []map[string]any) { maintenancePod(d)["automountServiceAccountToken"] = true }},
		"account-token-automount": {templates.ErrMaintenanceSubjects, func(d []map[string]any) {
			maintenanceObject(d, "ServiceAccount")["automountServiceAccountToken"] = true
		}},
		"no-projected-token": {templates.ErrMaintenanceTokenRequired, func(d []map[string]any) { delete(maintenancePod(d), "volumes") }},
		"unmounted-token": {templates.ErrMaintenanceTokenMounted, func(d []map[string]any) {
			delete(maintenancePod(d)["containers"].([]any)[0].(map[string]any), "volumeMounts")
		}},
		"legacy-account-alias": {templates.ErrMaintenanceServiceAccountName, func(d []map[string]any) { maintenancePod(d)["serviceAccount"] = "other" }},
		"long-running-projected-token": {templates.ErrMaintenanceJobsOnly, func(d []map[string]any) {
			job := maintenanceObject(d, "Job")
			job["kind"], job["apiVersion"] = "Deployment", "apps/v1"
		}},
		"unbound-additional-subject": {templates.ErrMaintenanceSubjectUnused, func(d []map[string]any) {
			binding := maintenanceObject(d, "RoleBinding")
			binding["subjects"] = append(binding["subjects"].([]any), map[string]any{"kind": "ServiceAccount", "name": "unused", "namespace": "test"})
		}},
		"transitive-broad-grant": {templates.ErrMaintenanceRoleRules, func(d []map[string]any) {
			role := maintenanceObject(d, "Role")
			role["rules"] = append(role["rules"].([]any), map[string]any{"apiGroups": []any{"apps"}, "resources": []any{"deployments"}, "resourceNames": []any{"app"}, "verbs": []any{"patch"}})
		}},
		"deployment-borrowing-bound-account": {templates.ErrMaintenanceJobsOnly, func(d []map[string]any) {
			pod := maintenanceObject(d, "Deployment")["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
			pod["serviceAccountName"] = "backup"
		}},
		"deployment-legacy-bound-account": {templates.ErrMaintenanceJobsOnly, func(d []map[string]any) {
			pod := maintenanceObject(d, "Deployment")["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
			pod["serviceAccount"] = "backup"
		}},
		"unused-role-and-binding": {templates.ErrMaintenanceRoleUnused, func(d []map[string]any) { maintenanceObject(d, "Job")["kind"] = "ConfigMap" }},
		"cluster-role":            {templates.ErrClusterScoped, func(d []map[string]any) { maintenanceObject(d, "Role")["kind"] = "ClusterRole" }},
		"cluster-role-binding":    {templates.ErrClusterScoped, func(d []map[string]any) { maintenanceObject(d, "RoleBinding")["kind"] = "ClusterRoleBinding" }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			docs, err := templates.DecodeDocuments([]byte(safeManifest + maintenanceManifest))
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(docs)
			if err := templates.Safety(maintenanceRecord(), maintenanceBytes(t, docs), "test"); !errors.Is(err, tc.rule) {
				t.Fatalf("got %v, want %v", err, tc.rule)
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
	if err := templates.Safety(r, []byte(safeManifest+maintenanceManifest), "test"); !errors.Is(err, templates.ErrMaintenanceRisk) {
		t.Fatalf("undisclosed maintenance API trust: got %v", err)
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
	if err := templates.Safety(maintenanceRecord(), maintenanceBytes(t, docs), "test"); !errors.Is(err, templates.ErrMaintenanceRoleRules) {
		t.Fatalf("a second RoleBinding granted broader transitive maintenance authority: %v", err)
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
	if err := templates.Safety(maintenanceRecord(), maintenanceBytes(t, docs), "test"); !errors.Is(err, templates.ErrMaintenanceTokenShape) {
		t.Fatalf("valid maintenance token hid another unrestricted token projection: %v", err)
	}
}
