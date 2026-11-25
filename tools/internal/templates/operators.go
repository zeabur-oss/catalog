package templates

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/kubeobj"
)

func hasProvider(r *Template, id string) bool {
	return slices.ContainsFunc(r.Requires, func(ref catalog.Ref) bool { return ref.Component == id })
}

// kindContext is one rendered object that a kindRule checks.
type kindContext struct {
	r         *Template
	d         map[string]any
	docs      []*unstructured.Unstructured
	namespace string
	where     string
	fail      func(string, Rule, ...string)
}

// kindRule is what the linter knows about the kinds an operator owns: a
// template may render them only when it requires the component that provides
// the operator, and check holds the rest of the rules for them.
type kindRule struct {
	group    string   // apiVersion
	kinds    []string // the kinds the rule covers, the ones the linter allows
	provider string   // the component the template must require
	missing  Rule     // the rule it breaks when it does not
	check    func(kindContext)
}

// kindRules has one entry for each operator kind.
var kindRules = []kindRule{
	{"postgresql.cnpg.io/v1", []string{"Cluster"}, "cloudnative-pg", ErrCNPGProvider, checkCNPGCluster},
	{"postgresql.cnpg.io/v1", []string{"Cluster", "Backup", "ScheduledBackup"}, "cloudnative-pg", ErrCNPGResourceProvider, nil},
	{"barmancloud.cnpg.io/v1", []string{"ObjectStore"}, "cloudnative-pg", ErrCNPGResourceProvider, checkBarmanObjectStore},
	{"psmdb.percona.com/v1", []string{"PerconaServerMongoDB"}, "percona-mongodb", ErrPerconaProvider, checkPercona},
	{"psmdb.percona.com/v1", []string{"PerconaServerMongoDBBackup"}, "percona-mongodb", ErrPerconaBackupProvider, checkPerconaBackup},
	{"dragonflydb.io/v1alpha1", []string{"Dragonfly"}, "dragonfly-operator", ErrDragonflyProvider, checkDragonfly},
	{"k8s.mariadb.com/v1alpha1", []string{"MariaDB", "Backup", "Database", "User", "Grant"}, "mariadb-operator", ErrMariaDBProvider, checkMariaDB},
	{"moco.cybozu.com/v1beta2", []string{"MySQLCluster"}, "moco", ErrMOCOProvider, checkMySQLCluster},
	{"traefik.io/v1alpha1", []string{"Middleware"}, "traefik", ErrMiddlewareProvider, checkMiddleware},
}

func (k kindRule) covers(api, kind string) bool {
	return k.group == api && slices.Contains(k.kinds, kind)
}

// operatorKind reports whether a kindRule covers the kind.
func operatorKind(api, kind string) bool {
	return slices.ContainsFunc(kindRules, func(k kindRule) bool { return k.covers(api, kind) })
}

// checkKinds applies the kindRules that cover the object.
func checkKinds(c kindContext) {
	api, _ := c.d["apiVersion"].(string)
	kind, _ := c.d["kind"].(string)
	for _, k := range kindRules {
		if !k.covers(api, kind) {
			continue
		}
		if !hasProvider(c.r, k.provider) {
			c.fail(c.where, k.missing)
		}
		if k.check != nil {
			k.check(c)
		}
	}
}

func checkCNPGCluster(c kindContext) {
	r, w, fail := c.r, c.where, c.fail
	spec := object(c.d["spec"])
	if !catalog.PinnedImage.MatchString(text(spec["imageName"])) {
		fail(w, "CNPG imageName must use sha256 digest")
	}
	checkResources(object(spec["resources"]), w, fail)
	if text(object(object(spec["inheritedMetadata"])["labels"])[TemplateLabel]) != r.ID {
		fail(w+"/inheritedMetadata", ErrTemplateLabel)
	}
	if len(r.Security.Risks) == 0 {
		fail(w, "operator-managed workload must document generated pod and service-account trust")
	}
	sec := text(object(spec["seccompProfile"])["type"])
	if sec != "RuntimeDefault" && sec != "Localhost" {
		fail(w, "CNPG restricted seccomp required")
	}
	for _, key := range []string{"postgresUID", "postgresGID"} {
		if v, ok := spec[key]; ok && fmt.Sprint(v) == "0" {
			fail(w, "CNPG root identity forbidden")
		}
	}
	for _, plugin := range array(spec["plugins"]) {
		if text(object(plugin)["name"]) != "barman-cloud.cloudnative-pg.io" {
			fail(w, "unsupported CNPG plugin forbidden")
		}
	}
}

func checkBarmanObjectStore(c kindContext) {
	spec := object(c.d["spec"])
	checkResources(object(object(spec["instanceSidecarConfiguration"])["resources"]), c.where+"/barman-sidecar", c.fail)
}

// safeOperandContext checks the security contexts that an operator copies
// into the pods it creates.
func safeOperandContext(p, c map[string]any, w string, fail func(string, Rule, ...string)) {
	for _, v := range []map[string]any{p, c} {
		if len(array(v["sysctls"])) > 0 {
			fail(w, "operand sysctl overrides forbidden")
		}
		if len(object(v["seLinuxOptions"])) > 0 {
			fail(w, "operand SELinux overrides require separate review")
		}
	}
	var spec corev1.PodSpec
	var sc corev1.SecurityContext
	if err := convert(c, &sc); err != nil {
		fail(w, ErrPodSecurity, err.Error())
		return
	}
	spec.Containers = []corev1.Container{{Name: "operand", SecurityContext: &sc}}
	if p != nil {
		var psc corev1.PodSecurityContext
		if err := convert(p, &psc); err != nil {
			fail(w, ErrPodSecurity, err.Error())
			return
		}
		spec.SecurityContext = &psc
	}
	checkPodSecurity(metav1.ObjectMeta{}, spec, "restricted", w, fail)
}

func checkPercona(c kindContext) {
	r, d, docs, w, fail := c.r, c.d, c.docs, c.where, c.fail
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
		if !catalog.PinnedImage.MatchString(text(image[1])) {
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
		if n < len(array(spec["replsets"])) {
			if text(object(p["labels"])[TemplateLabel]) != r.ID {
				fail(where, ErrTemplateLabel)
			}
			for _, key := range []string{"arbiter", "nonvoting", "hidden"} {
				if truth(object(p[key])["enabled"]) {
					fail(where, ErrUnsupportedOverride, key)
				}
			}
		}
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

func checkPerconaBackup(c kindContext) {
	d, docs, w, fail := c.d, c.docs, c.where, c.fail
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
		if doc.GetAPIVersion() != "psmdb.percona.com/v1" || doc.GetKind() != "PerconaServerMongoDB" || doc.GetName() != clusterName {
			continue
		}
		backup := object(object(doc.Object["spec"])["backup"])
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
func checkMiddleware(c kindContext) {
	d, w, fail := c.d, c.where, c.fail
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
// checkPodMetadata requires the TemplateLabel label in spec.podMetadata, which the
// operator copies to the pods it creates.
func checkPodMetadata(r *Template, spec map[string]any, w string, fail func(string, Rule, ...string)) {
	if text(object(object(spec["podMetadata"])["labels"])[TemplateLabel]) != r.ID {
		fail(w+"/podMetadata", ErrTemplateLabel)
	}
}

func onlyKeys(spec map[string]any, allowed []string, what, w string, fail func(string, Rule, ...string)) {
	for key := range spec {
		if !slices.Contains(allowed, key) {
			fail(w, ErrUnsupportedOverride, what+" "+key)
		}
	}
}

func declaredServiceAccount(docs []*unstructured.Unstructured, name string) bool {
	return slices.ContainsFunc(docs, func(doc *unstructured.Unstructured) bool {
		if doc.GetAPIVersion() != "v1" || doc.GetKind() != "ServiceAccount" || doc.GetName() != name {
			return false
		}
		var sa corev1.ServiceAccount
		return kubeobj.Typed(doc, &sa) == nil && sa.AutomountServiceAccountToken != nil && !*sa.AutomountServiceAccountToken
	})
}

func checkDragonfly(c kindContext) {
	r, d, docs, w, fail := c.r, c.d, c.docs, c.where, c.fail
	if len(r.Security.Risks) == 0 {
		fail(w, "security.risks must describe the BUSL-1.1 licence and the operator-generated pods")
	}
	spec := object(d["spec"])
	onlyKeys(spec, []string{"replicas", "image", "imagePullPolicy", "imagePullSecrets", "serviceAccountName", "args", "authentication", "env", "podSecurityContext", "containerSecurityContext", "podMetadata", "resources", "snapshot", "tlsSecretRef", "affinity", "nodeSelector", "tolerations", "topologySpreadConstraints"}, "Dragonfly", w, fail)
	if !catalog.PinnedImage.MatchString(text(spec["image"])) {
		fail(w, ErrDragonflyDigest)
	}
	checkPodMetadata(r, spec, w, fail)
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
func checkMariaDBRef(spec map[string]any, docs []*unstructured.Unstructured, namespace, w string, fail func(string, Rule, ...string)) {
	ref := object(spec["mariaDbRef"])
	onlyKeys(ref, []string{"name", "namespace", "waitForIt"}, "mariaDbRef", w, fail)
	if ns := text(ref["namespace"]); ns != "" && ns != namespace {
		fail(w, ErrMariaDBRefNamespace)
	}
	for _, doc := range docs {
		if doc.GetAPIVersion() == "k8s.mariadb.com/v1alpha1" && doc.GetKind() == "MariaDB" && doc.GetName() == text(ref["name"]) {
			return
		}
	}
	fail(w, ErrMariaDBRefUndeclared)
}

func checkMariaDB(c kindContext) {
	r, d, docs, namespace, w, fail := c.r, c.d, c.docs, c.namespace, c.where, c.fail
	spec := object(d["spec"])
	switch text(d["kind"]) {
	case "MariaDB":
		if len(r.Security.Risks) == 0 {
			fail(w, "security.risks must describe the operator-generated pods and helper images")
		}
		onlyKeys(spec, []string{"image", "imagePullPolicy", "imagePullSecrets", "replicas", "rootPasswordSecretKeyRef", "database", "username", "passwordSecretKeyRef", "serviceAccountName", "tls", "storage", "podMetadata", "podSecurityContext", "securityContext", "resources", "updateStrategy", "timeZone", "affinity", "nodeSelector", "tolerations", "topologySpreadConstraints"}, "MariaDB", w, fail)
		if !catalog.PinnedImage.MatchString(text(spec["image"])) {
			fail(w, "MariaDB image must use sha256 digest")
		}
		checkPodMetadata(r, spec, w, fail)
		checkResources(object(spec["resources"]), w, fail)
		safeOperandContext(object(spec["podSecurityContext"]), object(spec["securityContext"]), w, fail)
		if !declaredServiceAccount(docs, text(spec["serviceAccountName"])) {
			fail(w, "operand must reference declared token-disabled ServiceAccount")
		}
	case "Backup":
		checkMariaDBRef(spec, docs, namespace, w, fail)
		onlyKeys(spec, []string{"mariaDbRef", "schedule", "timeZone", "successfulJobsHistoryLimit", "failedJobsHistoryLimit", "serviceAccountName", "podMetadata", "podSecurityContext", "securityContext", "resources", "stagingStorage", "storage", "databases", "maxRetention", "compression", "backoffLimit"}, "backup", w, fail)
		checkPodMetadata(r, spec, w, fail)
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

// mocoContainers are the containers MOCO adds to each MySQL pod, which
// overwriteContainers may change.
// https://github.com/cybozu-go/moco/blob/v0.37.0/docs/customize-system-container.md
var mocoContainers = []string{"agent", "copy-moco-init", "moco-init", "slow-log"}

// checkMySQLCluster checks a MOCO MySQLCluster: the mysqld container in
// spec.podTemplate and the security contexts of the containers MOCO adds.
func checkMySQLCluster(c kindContext) {
	r, w, fail := c.r, c.where, c.fail
	if len(r.Security.Risks) == 0 {
		fail(w, "security.risks must describe the operator-generated pods and service account")
	}
	spec := object(c.d["spec"])
	onlyKeys(spec, []string{"replicas", "podTemplate", "volumeClaimTemplates", "mysqlConfigMapName", "maxDelaySeconds", "startupWaitSeconds", "disableSlowQueryLogContainer", "logRotationSchedule"}, "MySQLCluster", w, fail)
	tmpl := object(spec["podTemplate"])
	onlyKeys(tmpl, []string{"metadata", "spec", "overwriteContainers"}, "podTemplate", w, fail)
	if text(object(object(tmpl["metadata"])["labels"])[TemplateLabel]) != r.ID {
		fail(w+"/podTemplate", ErrTemplateLabel)
	}
	podSpec := object(tmpl["spec"])
	onlyKeys(podSpec, []string{"securityContext", "containers", "affinity", "nodeSelector", "tolerations", "topologySpreadConstraints", "terminationGracePeriodSeconds"}, "podTemplate.spec", w, fail)
	var pod corev1.PodSpec
	if err := convert(podSpec, &pod); err != nil {
		fail(w, ErrPodSecurity, err.Error())
		return
	}
	if len(pod.Containers) != 1 || pod.Containers[0].Name != "mysqld" {
		fail(w, ErrUnsupportedOverride, "containers other than mysqld")
	}
	for _, container := range array(podSpec["containers"]) {
		onlyKeys(object(container), []string{"name", "image", "resources", "securityContext"}, "container", w, fail)
	}
	for _, m := range pod.Containers {
		if !catalog.PinnedImage.MatchString(m.Image) {
			fail(w+"/"+m.Name, ErrImageDigest)
		}
		checkRequirements(m.Resources, w+"/"+m.Name, fail)
	}
	for _, o := range array(tmpl["overwriteContainers"]) {
		var added corev1.Container
		if err := convert(object(o), &added); err != nil || !slices.Contains(mocoContainers, added.Name) {
			fail(w, ErrUnsupportedOverride, "overwriteContainers "+text(object(o)["name"]))
			continue
		}
		pod.Containers = append(pod.Containers, corev1.Container{Name: added.Name, SecurityContext: added.SecurityContext})
	}
	// A container MOCO adds without an overwrite runs with MOCO's default
	// security context, which restricted does not admit.
	for _, name := range mocoContainers {
		if !slices.ContainsFunc(pod.Containers, func(c corev1.Container) bool { return c.Name == name }) {
			pod.Containers = append(pod.Containers, corev1.Container{Name: name})
		}
	}
	checkPodSecurity(metav1.ObjectMeta{}, pod, "restricted", w, fail)
}
