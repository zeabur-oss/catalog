package templates

import (
	"fmt"
	"net/netip"
	"slices"
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
func safeOperandContext(p, c map[string]any, w string, fail func(string, Rule, ...string)) {
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
			fail(w, ErrOperandUIDZero)
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
func checkPercona(r *Template, d map[string]any, docs []map[string]any, w string, fail func(string, Rule, ...string)) {
	if !hasProvider(r, "databases/percona-mongodb") {
		fail(w, "Percona operand requires exact databases/percona-mongodb")
	}
	if len(r.Security.Risks) == 0 {
		fail(w, "security.risks must describe the operator-generated pods, service account and probes")
	}
	spec := object(d["spec"])
	backup := object(spec["backup"])
	images := [][2]any{{"image", spec["image"]}, {"initImage", spec["initImage"]}}
	if truth(backup["enabled"]) {
		images = append(images, [2]any{"backup.image", backup["image"]})
		for name, storage := range object(backup["storages"]) {
			if !perconaStorageSafe(object(storage)) {
				fail(w, ErrPerconaBackupStorage, name)
			}
		}
	}
	for _, image := range images {
		if !digestImage.MatchString(text(image[1])) {
			fail(w, "Percona image must use sha256 digest", text(image[0]))
		}
	}
	if truth(object(spec["sharding"])["enabled"]) {
		fail(w, "sharded Percona topology requires separate safety adapter")
	}
	for key, v := range object(spec["unsafeFlags"]) {
		if truth(v) && key != "replsetSize" {
			fail(w, "unsafe Percona flag forbidden", key)
		}
	}
	safeOperandContext(nil, object(spec["initContainerSecurityContext"]), w+"/init", fail)
	parts := array(spec["replsets"])
	if len(parts) == 0 {
		fail(w, "at least one Percona replica set required")
	}
	if truth(backup["enabled"]) {
		parts = append(parts, backup)
	}
	for n, part := range parts {
		p := object(part)
		where := fmt.Sprintf("%s/operand-%d", w, n)
		checkResources(object(p["resources"]), where, fail)
		safeOperandContext(object(p["podSecurityContext"]), object(p["containerSecurityContext"]), where, fail)
		if !declaredServiceAccount(docs, text(p["serviceAccountName"])) {
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
				fail(where, ErrUnsupportedOverride, key)
			}
		}
	}
}

func checkPerconaBackup(r *Template, d map[string]any, docs []map[string]any, w string, fail func(string, Rule, ...string)) {
	if !hasProvider(r, "databases/percona-mongodb") {
		fail(w, ErrPerconaBackupProvider)
	}
	spec := object(d["spec"])
	for key := range spec {
		switch key {
		case "clusterName", "storageName", "type", "compressionType", "startingDeadlineSeconds":
		default:
			fail(w, ErrPerconaBackupOverride, key)
		}
	}
	if text(spec["type"]) != "logical" || text(spec["compressionType"]) != "gzip" {
		fail(w, ErrPerconaBackupMode)
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
		if !truth(backup["enabled"]) || !perconaStorageSafe(object(object(backup["storages"])[storageName])) {
			fail(w, ErrPerconaBackupStorage)
		}
		return
	}
	fail(w, ErrPerconaBackupCluster)
}

func perconaStorageSafe(storage map[string]any) bool {
	s3 := object(storage["s3"])
	return text(storage["type"]) == "s3" && text(s3["credentialsSecret"]) != "" && strings.HasPrefix(text(s3["endpointUrl"]), "https://") && !truth(s3["insecureSkipTLSVerify"])
}
func checkMiddleware(r *Template, d map[string]any, w string, fail func(string, Rule, ...string)) {
	if !hasProvider(r, "ingress/traefik") {
		fail(w, "Middleware requires ingress/traefik")
	}
	spec := object(d["spec"])
	if len(spec) != 1 || spec["ipAllowList"] == nil {
		fail(w, ErrIPAllowListOnly)
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
			fail(w, ErrIPAllowListCIDR)
		}
	}
}

// onlyKeys rejects each spec field that is not in allowed.
func onlyKeys(spec map[string]any, allowed []string, what, w string, fail func(string, Rule, ...string)) {
	for key := range spec {
		if !slices.Contains(allowed, key) {
			fail(w, ErrUnsupportedOverride, what+" "+key)
		}
	}
}

func declaredServiceAccount(docs []map[string]any, name string) bool {
	for _, doc := range docs {
		if text(doc["kind"]) == "ServiceAccount" && text(object(doc["metadata"])["name"]) == name && doc["automountServiceAccountToken"] == false {
			return true
		}
	}
	return false
}

func checkDragonfly(r *Template, d map[string]any, docs []map[string]any, w string, fail func(string, Rule, ...string)) {
	if !hasProvider(r, "databases/dragonfly-operator") {
		fail(w, ErrDragonflyProvider)
	}
	if len(r.Security.Risks) == 0 {
		fail(w, "security.risks must describe the BUSL-1.1 licence and the operator-generated pods")
	}
	spec := object(d["spec"])
	onlyKeys(spec, []string{"replicas", "image", "imagePullPolicy", "imagePullSecrets", "serviceAccountName", "args", "authentication", "env", "podSecurityContext", "containerSecurityContext", "resources", "snapshot", "tlsSecretRef", "affinity", "nodeSelector", "tolerations", "topologySpreadConstraints"}, "Dragonfly", w, fail)
	if !digestImage.MatchString(text(spec["image"])) {
		fail(w, ErrDragonflyDigest)
	}
	checkResources(object(spec["resources"]), w, fail)
	safeOperandContext(object(spec["podSecurityContext"]), object(spec["containerSecurityContext"]), w, fail)
	if !declaredServiceAccount(docs, text(spec["serviceAccountName"])) {
		fail(w, "operand must reference declared token-disabled ServiceAccount")
	}
	auth := object(object(spec["authentication"])["passwordFromSecret"])
	if text(auth["name"]) == "" || text(auth["key"]) == "" {
		fail(w, ErrDragonflyPassword)
	}
	// These flags open a port or the admin console without a password.
	for _, arg := range array(spec["args"]) {
		for _, flag := range []string{"--admin_nopass", "--memcached_port"} {
			if text(arg) == flag || strings.HasPrefix(text(arg), flag+"=") {
				fail(w, ErrDragonflyFlag, flag)
			}
		}
	}
}

// checkMariaDBRef requires a reference to a MariaDB in this template. The operator
// is cluster-wide, so a reference to another namespace reaches another tenant.
func checkMariaDBRef(spec map[string]any, docs []map[string]any, namespace, w string, fail func(string, Rule, ...string)) {
	ref := object(spec["mariaDbRef"])
	onlyKeys(ref, []string{"name", "namespace", "waitForIt"}, "mariaDbRef", w, fail)
	if ns := text(ref["namespace"]); ns != "" && ns != namespace {
		fail(w, ErrMariaDBRefNamespace)
	}
	for _, doc := range docs {
		if text(doc["apiVersion"]) == "k8s.mariadb.com/v1alpha1" && text(doc["kind"]) == "MariaDB" && text(object(doc["metadata"])["name"]) == text(ref["name"]) {
			return
		}
	}
	fail(w, ErrMariaDBRefUndeclared)
}

func checkMariaDB(r *Template, d map[string]any, docs []map[string]any, namespace, w string, fail func(string, Rule, ...string)) {
	if !hasProvider(r, "databases/mariadb-operator") {
		fail(w, ErrMariaDBProvider)
	}
	spec := object(d["spec"])
	switch text(d["kind"]) {
	case "MariaDB":
		if len(r.Security.Risks) == 0 {
			fail(w, "security.risks must describe the operator-generated pods and helper images")
		}
		onlyKeys(spec, []string{"image", "imagePullPolicy", "imagePullSecrets", "replicas", "rootPasswordSecretKeyRef", "database", "username", "passwordSecretKeyRef", "serviceAccountName", "tls", "storage", "podSecurityContext", "securityContext", "resources", "updateStrategy", "timeZone", "affinity", "nodeSelector", "tolerations", "topologySpreadConstraints"}, "MariaDB", w, fail)
		if !digestImage.MatchString(text(spec["image"])) {
			fail(w, "MariaDB image must use sha256 digest")
		}
		checkResources(object(spec["resources"]), w, fail)
		safeOperandContext(object(spec["podSecurityContext"]), object(spec["securityContext"]), w, fail)
		if !declaredServiceAccount(docs, text(spec["serviceAccountName"])) {
			fail(w, "operand must reference declared token-disabled ServiceAccount")
		}
	case "Backup":
		checkMariaDBRef(spec, docs, namespace, w, fail)
		onlyKeys(spec, []string{"mariaDbRef", "schedule", "timeZone", "successfulJobsHistoryLimit", "failedJobsHistoryLimit", "serviceAccountName", "podMetadata", "podSecurityContext", "securityContext", "resources", "stagingStorage", "storage", "databases", "maxRetention", "compression", "backoffLimit"}, "backup", w, fail)
		checkResources(object(spec["resources"]), w, fail)
		safeOperandContext(object(spec["podSecurityContext"]), object(spec["securityContext"]), w, fail)
		if !declaredServiceAccount(docs, text(spec["serviceAccountName"])) {
			fail(w, "operand must reference declared token-disabled ServiceAccount")
		}
		for key := range object(spec["stagingStorage"]) {
			if key != "persistentVolumeClaim" {
				fail(w, "backup stagingStorage must use PVC only")
			}
		}
		storage := object(spec["storage"])
		s3 := object(storage["s3"])
		if len(storage) != 1 || text(s3["bucket"]) == "" || text(s3["endpoint"]) == "" || len(object(s3["accessKeyIdSecretKeyRef"])) == 0 || len(object(s3["secretAccessKeySecretKeyRef"])) == 0 || !truth(object(s3["tls"])["enabled"]) {
			fail(w, ErrMariaDBBackupS3)
		}
	case "Database":
		checkMariaDBRef(spec, docs, namespace, w, fail)
		onlyKeys(spec, []string{"mariaDbRef", "name", "characterSet", "collate", "cleanupPolicy", "requeueInterval", "retryInterval"}, "Database", w, fail)
	case "User":
		checkMariaDBRef(spec, docs, namespace, w, fail)
		onlyKeys(spec, []string{"mariaDbRef", "name", "host", "passwordSecretKeyRef", "require", "maxUserConnections", "cleanupPolicy", "requeueInterval", "retryInterval"}, "User", w, fail)
		ref := object(spec["passwordSecretKeyRef"])
		if text(ref["name"]) == "" || text(ref["key"]) == "" {
			fail(w, "User passwordSecretKeyRef required")
		}
	case "Grant":
		checkMariaDBRef(spec, docs, namespace, w, fail)
		onlyKeys(spec, []string{"mariaDbRef", "username", "host", "database", "table", "privileges", "grantOption", "cleanupPolicy", "requeueInterval", "retryInterval"}, "Grant", w, fail)
		if db := text(spec["database"]); db == "" || db == "*" {
			fail(w, ErrGlobalGrant)
		}
		if truth(spec["grantOption"]) {
			fail(w, "Grant grantOption forbidden")
		}
	}
}
