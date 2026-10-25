package templates

import (
	"regexp"
	"strings"
)

var maintenanceTokenPath = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// checkMaintenance admits only namespaced, explicitly named Deployment scale
// grants consumed by token-off Jobs or CronJobs. The returned workload keys are
// the only exceptions to checkPod's projected-token prohibition. A failed
// validation always reaches Safety's fail callback; this is not an RBAC bypass.
func checkMaintenance(r *Template, docs []map[string]any, namespace string, fail func(string, Rule, ...string)) map[string]bool {
	deployments := map[string]bool{}
	accounts := map[string]bool{}
	roles := map[string]bool{}
	inNamespace := func(d map[string]any) bool {
		ns := text(object(d["metadata"])["namespace"])
		return ns == "" || ns == namespace
	}
	for _, d := range docs {
		if !inNamespace(d) {
			continue
		}
		name := text(object(d["metadata"])["name"])
		switch text(d["apiVersion"]) + "/" + text(d["kind"]) {
		case "apps/v1/Deployment":
			if name != "" {
				deployments[name] = true
			}
		case "v1/ServiceAccount":
			if name != "" && d["automountServiceAccountToken"] == false {
				accounts[name] = true
			}
		}
	}
	for _, d := range docs {
		if text(d["kind"]) != "Role" {
			continue
		}
		name := text(object(d["metadata"])["name"])
		roles[name] = validMaintenanceRole(d, namespace, deployments, fail)
	}
	type binding struct {
		where    string
		role     string
		accounts []string
	}
	var bindings []binding
	bound := map[string]bool{}
	for _, d := range docs {
		if text(d["kind"]) != "RoleBinding" {
			continue
		}
		where := "RoleBinding/" + text(object(d["metadata"])["name"])
		valid := true
		reject := func(rule Rule) { valid = false; fail(where, rule) }
		if text(d["apiVersion"]) != "rbac.authorization.k8s.io/v1" || text(object(d["metadata"])["namespace"]) != namespace || text(object(d["metadata"])["name"]) == "" {
			reject(ErrMaintenanceBindingScope)
		}
		if !maintenanceFields(d, "apiVersion", "kind", "metadata", "roleRef", "subjects") {
			reject("unsupported maintenance RoleBinding fields forbidden")
		}
		ref := object(d["roleRef"])
		role := text(ref["name"])
		if !maintenanceFields(ref, "apiGroup", "kind", "name") || text(ref["apiGroup"]) != "rbac.authorization.k8s.io" || text(ref["kind"]) != "Role" || !roles[role] {
			reject(ErrMaintenanceBindingRole)
		}
		subjects := array(d["subjects"])
		if len(subjects) == 0 {
			reject("maintenance RoleBinding requires declared ServiceAccount subjects")
		}
		var names []string
		for _, raw := range subjects {
			subject := object(raw)
			name := text(subject["name"])
			if !maintenanceFields(subject, "kind", "name", "namespace", "apiGroup") || text(subject["kind"]) != "ServiceAccount" || text(subject["namespace"]) != namespace || text(subject["apiGroup"]) != "" || !accounts[name] {
				reject(ErrMaintenanceSubjects)
			}
			names = append(names, name)
		}
		if valid {
			bindings = append(bindings, binding{where: where, role: role, accounts: names})
			for _, name := range names {
				bound[name] = true
			}
		}
	}
	allowed := map[string]bool{}
	consumed := map[string]bool{}
	for _, d := range docs {
		p := maintenancePodSpec(d)
		if p == nil {
			continue
		}
		where := text(d["kind"]) + "/" + text(object(d["metadata"])["name"])
		sa := text(p["serviceAccountName"])
		if sa == "" {
			sa = text(p["serviceAccount"])
		}
		projected := maintenanceHasToken(p)
		if !projected && !bound[sa] {
			continue
		}
		valid := true
		reject := func(rule Rule) { valid = false; fail(where, rule) }
		kind := text(d["kind"])
		if text(d["apiVersion"]) != "batch/v1" || (kind != "Job" && kind != "CronJob") {
			reject(ErrMaintenanceJobsOnly)
		}
		if !inNamespace(d) || !bound[sa] || !accounts[sa] {
			reject(ErrMaintenanceTokenBinding)
		}
		if p["automountServiceAccountToken"] != false {
			reject(ErrMaintenancePodAutomount)
		}
		if _, legacy := p["serviceAccount"]; legacy {
			reject(ErrMaintenanceServiceAccountName)
		}
		if !projected {
			reject(ErrMaintenanceTokenRequired)
		}
		if len(r.Security.Risks) == 0 {
			reject(ErrMaintenanceRisk)
		}
		for _, raw := range array(p["volumes"]) {
			volume := object(raw)
			for _, source := range array(object(volume["projected"])["sources"]) {
				token, exists := object(source)["serviceAccountToken"]
				if !exists {
					continue
				}
				spec := object(token)
				expiry, integer := spec["expirationSeconds"].(int)
				if !maintenanceFields(spec, "path", "expirationSeconds", "audience") || !integer || expiry < 600 || expiry > 3600 || !maintenanceTokenPath.MatchString(text(spec["path"])) {
					reject(ErrMaintenanceTokenShape)
				}
				if audience, exists := spec["audience"]; exists && audience != "" {
					reject(ErrMaintenanceTokenAudience)
				}
				mounted := false
				for _, key := range []string{"containers", "initContainers"} {
					for _, container := range array(p[key]) {
						for _, mount := range array(object(container)["volumeMounts"]) {
							if text(volume["name"]) != "" && text(object(mount)["name"]) == text(volume["name"]) {
								mounted = true
							}
						}
					}
				}
				if !mounted {
					reject(ErrMaintenanceTokenMounted)
				}
			}
		}
		if valid {
			allowed[where] = true
			consumed[sa] = true
		}
	}
	usedRoles := map[string]bool{}
	for _, b := range bindings {
		for _, sa := range b.accounts {
			if !consumed[sa] {
				fail(b.where, ErrMaintenanceSubjectUnused)
			} else {
				usedRoles[b.role] = true
			}
		}
	}
	for role, valid := range roles {
		if valid && !usedRoles[role] {
			fail("Role/"+role, ErrMaintenanceRoleUnused)
		}
	}
	return allowed
}

func validMaintenanceRole(d map[string]any, namespace string, deployments map[string]bool, fail func(string, Rule, ...string)) bool {
	where := "Role/" + text(object(d["metadata"])["name"])
	valid := true
	reject := func(rule Rule) { valid = false; fail(where, rule) }
	if text(d["apiVersion"]) != "rbac.authorization.k8s.io/v1" || text(object(d["metadata"])["namespace"]) != namespace || text(object(d["metadata"])["name"]) == "" {
		reject(ErrMaintenanceRoleScope)
	}
	if !maintenanceFields(d, "apiVersion", "kind", "metadata", "rules") {
		reject("unsupported maintenance Role fields forbidden")
	}
	rules := array(d["rules"])
	if len(rules) == 0 {
		reject("maintenance Role requires named Deployment scale rules")
	}
	for _, raw := range rules {
		rule := object(raw)
		groups, resources := array(rule["apiGroups"]), array(rule["resources"])
		if !maintenanceFields(rule, "apiGroups", "resources", "verbs", "resourceNames") || len(groups) != 1 || groups[0] != "apps" || len(resources) != 1 || resources[0] != "deployments/scale" {
			reject(ErrMaintenanceRoleRules)
		}
		verbs := array(rule["verbs"])
		if len(verbs) == 0 {
			reject(ErrMaintenanceVerbs)
		}
		for _, verb := range verbs {
			if verb != "get" && verb != "update" && verb != "patch" {
				reject(ErrMaintenanceVerbs)
			}
		}
		names := array(rule["resourceNames"])
		if len(names) == 0 {
			reject(ErrMaintenanceResourceNames)
		}
		for _, name := range names {
			if strings.ContainsAny(text(name), "*?") || !deployments[text(name)] {
				reject(ErrMaintenanceResourceNames)
			}
		}
	}
	return valid
}

func maintenanceFields(m map[string]any, allowed ...string) bool {
	for key := range m {
		found := false
		for _, name := range allowed {
			if key == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func maintenancePodSpec(d map[string]any) map[string]any {
	spec := object(d["spec"])
	switch text(d["kind"]) {
	case "Pod":
		return spec
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job":
		return object(object(spec["template"])["spec"])
	case "CronJob":
		return object(object(object(object(spec["jobTemplate"])["spec"])["template"])["spec"])
	default:
		return nil
	}
}

func maintenanceHasToken(p map[string]any) bool {
	for _, volume := range array(p["volumes"]) {
		for _, source := range array(object(object(volume)["projected"])["sources"]) {
			if _, exists := object(source)["serviceAccountToken"]; exists {
				return true
			}
		}
	}
	return false
}
