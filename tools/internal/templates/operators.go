package templates

import (
	"fmt"
	"net/netip"
	"strings"
)

func hasProvider(r *Template, id string) bool {
	for _, ref := range r.Requires {
		if ref == id {
			return true
		}
	}
	return false
}
func safeOperandContext(p, c map[string]any, w string, fail func(string, string)) {
	if truth(c["privileged"]) || truth(object(c["windowsOptions"])["hostProcess"]) || truth(object(p["windowsOptions"])["hostProcess"]) {
		fail(w, "privileged operator operand forbidden")
	}
	if c["allowPrivilegeEscalation"] != false {
		fail(w, "operand allowPrivilegeEscalation must be false")
	}
	nonroot, ok := c["runAsNonRoot"]
	if !ok {
		nonroot = p["runAsNonRoot"]
	}
	if nonroot != true {
		fail(w, "operand effective runAsNonRoot must be true")
	}
	for _, v := range []map[string]any{p, c} {
		if uid, ok := v["runAsUser"]; ok && fmt.Sprint(uid) == "0" {
			fail(w, "operand UID zero forbidden")
		}
		if text(object(v["seccompProfile"])["type"]) == "Unconfined" {
			fail(w, "operand unconfined seccomp forbidden")
		}
		if text(object(v["appArmorProfile"])["type"]) == "Unconfined" {
			fail(w, "operand unconfined AppArmor forbidden")
		}
		if len(array(v["sysctls"])) > 0 {
			fail(w, "operand sysctl overrides forbidden")
		}
		if len(object(v["seLinuxOptions"])) > 0 {
			fail(w, "operand SELinux overrides require separate review")
		}
	}
	sec := text(object(c["seccompProfile"])["type"])
	if sec == "" {
		sec = text(object(p["seccompProfile"])["type"])
	}
	if sec != "RuntimeDefault" && sec != "Localhost" {
		fail(w, "operand restricted seccomp required")
	}
	caps := object(c["capabilities"])
	if !contains(array(caps["drop"]), "ALL") {
		fail(w, "operand capabilities.drop ALL required")
	}
	for _, v := range array(caps["add"]) {
		if v != "NET_BIND_SERVICE" {
			fail(w, "operand elevated capability forbidden")
		}
	}
}
func checkPercona(r *Template, d map[string]any, docs []map[string]any, w string, fail func(string, string)) {
	if !hasProvider(r, "databases/percona-mongodb") {
		fail(w, "Percona operand requires exact databases/percona-mongodb")
	}
	if len(r.Security.Risks) == 0 {
		fail(w, "security.risks must describe the operator-generated pods, service account and probes")
	}
	spec := object(d["spec"])
	for _, entry := range []struct {
		name  string
		value any
	}{{"image", spec["image"]}, {"initImage", spec["initImage"]}, {"backup.image", object(spec["backup"])["image"]}} {
		if !digestImage.MatchString(text(entry.value)) {
			fail(w, "Percona "+entry.name+" must use sha256 digest")
		}
	}
	if truth(object(spec["sharding"])["enabled"]) {
		fail(w, "sharded Percona topology requires separate safety adapter")
	}
	for key, v := range object(spec["unsafeFlags"]) {
		if truth(v) && key != "replsetSize" {
			fail(w, "unsafe Percona flag forbidden: "+key)
		}
	}
	safeOperandContext(nil, object(spec["initContainerSecurityContext"]), w+"/init", fail)
	parts := array(spec["replsets"])
	if len(parts) == 0 {
		fail(w, "at least one Percona replica set required")
	}
	parts = append(parts, object(spec["backup"]))
	for n, part := range parts {
		p := object(part)
		where := fmt.Sprintf("%s/operand-%d", w, n)
		checkResources(object(p["resources"]), where, fail)
		safeOperandContext(object(p["podSecurityContext"]), object(p["containerSecurityContext"]), where, fail)
		sa := text(p["serviceAccountName"])
		found := false
		for _, doc := range docs {
			if text(doc["kind"]) == "ServiceAccount" && text(object(doc["metadata"])["name"]) == sa && doc["automountServiceAccountToken"] == false {
				found = true
			}
		}
		if !found {
			fail(where, "operand must reference declared token-disabled ServiceAccount")
		}
		volume := object(p["volumeSpec"])
		for key := range volume {
			if key != "persistentVolumeClaim" {
				fail(where, "operand volumeSpec must use PVC only")
			}
		}
		for _, key := range []string{"sidecars", "sidecarVolumes", "hostAliases", "initContainers"} {
			if len(array(p[key])) > 0 {
				fail(where, "unsupported operand override forbidden: "+key)
			}
		}
	}
}

func checkPerconaBackup(r *Template, d map[string]any, docs []map[string]any, w string, fail func(string, string)) {
	if !hasProvider(r, "databases/percona-mongodb") {
		fail(w, "Percona backup requires exact databases/percona-mongodb")
	}
	spec := object(d["spec"])
	for key := range spec {
		switch key {
		case "clusterName", "storageName", "type", "compressionType", "startingDeadlineSeconds":
		default:
			fail(w, "unsupported backup override forbidden: "+key)
		}
	}
	if text(spec["type"]) != "logical" || text(spec["compressionType"]) != "gzip" {
		fail(w, "only gzip-compressed logical Percona backups supported")
	}
	clusterName, storageName := text(spec["clusterName"]), text(spec["storageName"])
	if clusterName == "" || storageName == "" {
		fail(w, "backup requires an explicit cluster and storage")
		return
	}
	for _, doc := range docs {
		if text(doc["apiVersion"]) != "psmdb.percona.com/v1" || text(doc["kind"]) != "PerconaServerMongoDB" || text(object(doc["metadata"])["name"]) != clusterName {
			continue
		}
		backup := object(object(doc["spec"])["backup"])
		storage := object(object(backup["storages"])[storageName])
		s3 := object(storage["s3"])
		if !truth(backup["enabled"]) || text(storage["type"]) != "s3" || text(s3["credentialsSecret"]) == "" || !strings.HasPrefix(text(s3["endpointUrl"]), "https://") || truth(s3["insecureSkipTLSVerify"]) {
			fail(w, "backup must use the declared cluster's enabled S3 storage with explicit credentials and TLS verification")
		}
		return
	}
	fail(w, "backup cluster must be declared in this template")
}
func checkMiddleware(r *Template, d map[string]any, w string, fail func(string, string)) {
	if !hasProvider(r, "ingress/traefik") {
		fail(w, "Middleware requires ingress/traefik")
	}
	spec := object(d["spec"])
	if len(spec) != 1 || spec["ipAllowList"] == nil {
		fail(w, "only explicit ipAllowList Middleware supported")
		return
	}
	allow := object(spec["ipAllowList"])
	if len(allow) != 1 {
		fail(w, "ipAllowList may only contain sourceRange")
	}
	ranges := array(allow["sourceRange"])
	if len(ranges) == 0 {
		fail(w, "ipAllowList sourceRange required")
	}
	for _, v := range ranges {
		cidr, e := netip.ParsePrefix(text(v))
		if e != nil || cidr.Bits() == 0 || strings.TrimSpace(text(v)) != text(v) {
			fail(w, "ipAllowList requires non-public, valid explicit CIDR")
		}
	}
}
