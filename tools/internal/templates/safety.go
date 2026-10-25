package templates

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func text(v any) string           { s, _ := v.(string); return s }
func truth(v any) bool            { b, _ := v.(bool); return b }
func contains(a []any, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

var digestImage = regexp.MustCompile(`^[^\s]+@sha256:[a-f0-9]{64}$`)
var resourceQuantity = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)(?:m|[KMGTPE]i?|[kMGTPE]|[eE][+-]?[0-9]+)?$`)

// Rule is a safety rule that a rendered object breaks. Safety wraps each one it finds.
type Rule string

func (r Rule) Error() string { return string(r) }

// Rules that callers can match with errors.Is.
const (
	ErrNamespaceEscape       Rule = "resource escapes target namespace"
	ErrClusterScoped         Rule = "cluster-scoped resources and RBAC are forbidden"
	ErrTraefikProvider       Rule = "Traefik Ingress requires exact ingress/traefik provider"
	ErrClusterIssuerProvider Rule = "ClusterIssuer annotation requires exact certificates/cert-manager provider"
	ErrCNPGProvider          Rule = "CNPG workload requires exact databases/cloudnative-pg provider"
	ErrDefaultDeny           Rule = "a NetworkPolicy <id>-default-deny that selects the template's pods and denies only ingress is required"
	ErrTemplateLabel         Rule = "pods must have the label " + TemplateLabel + " set to the template ID"
	ErrForeignPods           Rule = "NetworkPolicy selectors must select only pods with the label " + TemplateLabel + " set to the template ID"
	ErrPositiveResource      Rule = "positive resource required"
	ErrPodTokenAutomount     Rule = "automountServiceAccountToken must explicitly be false"
	ErrPodSecurity           Rule = "pod breaks the Pod Security Standard of its service"
	ErrUnsafeSysctl          Rule = "sysctl not in safe allowlist"
	ErrImageDigest           Rule = "image must use immutable sha256 digest"
	ErrProbesRequired        Rule = "readiness and liveness probes required"
	ErrIPAllowListOnly       Rule = "only explicit ipAllowList Middleware supported"
	ErrIPAllowListCIDR       Rule = "ipAllowList requires non-public, valid explicit CIDR"
	ErrUnsupportedOverride   Rule = "unsupported override forbidden"
	ErrDragonflyProvider     Rule = "Dragonfly requires exact databases/dragonfly-operator"
	ErrDragonflyDigest       Rule = "Dragonfly image must use sha256 digest"
	ErrDragonflyPassword     Rule = "Dragonfly authentication.passwordFromSecret required"
	ErrDragonflyFlag         Rule = "unauthenticated Dragonfly flag forbidden"
	ErrMariaDBRefNamespace   Rule = "mariaDbRef escapes target namespace"
	ErrMariaDBRefUndeclared  Rule = "mariaDbRef must name a MariaDB declared in this template"
	ErrMariaDBProvider       Rule = "MariaDB Operator resources require exact databases/mariadb-operator"
	ErrMariaDBBackupS3       Rule = "backup must use S3 with explicit credentials and TLS verification"
	ErrGlobalGrant           Rule = "Grant must name one database; global privileges forbidden"
	ErrPerconaBackupProvider Rule = "Percona backup requires exact databases/percona-mongodb"
	ErrPerconaBackupOverride Rule = "unsupported backup override forbidden"
	ErrPerconaBackupMode     Rule = "only gzip-compressed logical Percona backups supported"
	ErrPerconaBackupStorage  Rule = "backup must use the declared cluster's enabled S3 storage with explicit credentials and TLS verification"
	ErrPerconaBackupCluster  Rule = "backup cluster must be declared in this template"
	ErrUnsupportedKind       Rule = "unsupported resource kind; the linter needs a namespaced safety adapter for it"
	ErrStorageClass          Rule = "storage class forbidden; volumes use the cluster's default StorageClass"
)

// TemplateLabel is the label that every pod of a template has, with the template
// ID as its value. The NetworkPolicies of a template select only these pods, so
// other pods in a shared namespace keep their own policies. It is not
// app.kubernetes.io/part-of: the Dragonfly operator owns that label and
// rejects it in podMetadata.
const TemplateLabel = "templates.zeabur.com/template"

// ownPods reports whether sel selects only the pods of template id.
func ownPods(sel map[string]any, id string) bool {
	return id != "" && text(object(sel["matchLabels"])[TemplateLabel]) == id
}

// Safety checks the rendered objects themselves.
func Safety(r *Template, b []byte, namespace string) error {
	docs, err := DecodeDocuments(b)
	if err != nil {
		return err
	}
	var errs []error
	defaultDeny := false
	fail := func(where string, rule Rule, detail ...string) {
		err := fmt.Errorf("%s: %w", where, rule)
		if len(detail) > 0 {
			err = fmt.Errorf("%w: %s", err, strings.Join(detail, " "))
		}
		errs = append(errs, err)
	}
	seen := map[string]bool{}
	for _, d := range docs {
		k := text(d["kind"])
		api := text(d["apiVersion"])
		meta := object(d["metadata"])
		name := text(meta["name"])
		where := k + "/" + name
		key := api + "/" + where
		if seen[key] {
			fail(where, "duplicate resource")
		}
		seen[key] = true
		ns := text(meta["namespace"])
		if ns != "" && ns != namespace {
			fail(where, ErrNamespaceEscape)
		}
		switch k {
		case "Namespace", "Node", "PersistentVolume", "StorageClass", "CustomResourceDefinition", "ClusterRole", "ClusterRoleBinding", "MutatingWebhookConfiguration", "ValidatingWebhookConfiguration", "APIService", "PriorityClass", "CertificateSigningRequest":
			fail(where, ErrClusterScoped)
		}
		switch api + "/" + k {
		case "v1/Pod", "v1/Service", "v1/ConfigMap", "v1/Secret", "v1/ServiceAccount", "v1/PersistentVolumeClaim", "apps/v1/Deployment", "apps/v1/StatefulSet", "apps/v1/DaemonSet", "apps/v1/ReplicaSet", "batch/v1/Job", "batch/v1/CronJob", "networking.k8s.io/v1/Ingress", "networking.k8s.io/v1/NetworkPolicy", "policy/v1/PodDisruptionBudget", "autoscaling/v2/HorizontalPodAutoscaler", "cert-manager.io/v1/Certificate", "postgresql.cnpg.io/v1/Cluster", "postgresql.cnpg.io/v1/Backup", "postgresql.cnpg.io/v1/ScheduledBackup", "barmancloud.cnpg.io/v1/ObjectStore", "psmdb.percona.com/v1/PerconaServerMongoDB", "traefik.io/v1alpha1/Middleware":
		case "psmdb.percona.com/v1/PerconaServerMongoDBBackup":
		case "dragonflydb.io/v1alpha1/Dragonfly":
		case "k8s.mariadb.com/v1alpha1/MariaDB", "k8s.mariadb.com/v1alpha1/Backup", "k8s.mariadb.com/v1alpha1/Database", "k8s.mariadb.com/v1alpha1/User", "k8s.mariadb.com/v1alpha1/Grant":
		default:
			fail(where, ErrUnsupportedKind)
		}
		if hasKey(d, "storageClassName", "storageClass") {
			fail(where, ErrStorageClass)
		}
		if k == "Secret" {
			fail(where, "rendered Secrets forbidden; use explicit inputs and preexisting Secrets")
		}
		if k == "ServiceAccount" && d["automountServiceAccountToken"] != false {
			fail(where, "ServiceAccount must disable token automount")
		}
		if k == "Service" {
			s := object(d["spec"])
			if text(s["type"]) == "NodePort" || text(s["type"]) == "LoadBalancer" || len(array(s["externalIPs"])) > 0 {
				fail(where, "externally exposed Service forbidden; use ingress")
			}
		}
		spec := object(d["spec"])
		if k == "Ingress" {
			if text(spec["ingressClassName"]) == "traefik" && !hasProvider(r, "ingress/traefik") {
				fail(where, ErrTraefikProvider)
			}
			if text(object(meta["annotations"])["cert-manager.io/cluster-issuer"]) != "" && !hasProvider(r, "certificates/cert-manager") {
				fail(where, ErrClusterIssuerProvider)
			}
		}
		var p, pm map[string]any
		switch k {
		case "Pod":
			p = spec
			pm = meta
		case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job":
			t := object(spec["template"])
			p = object(t["spec"])
			pm = object(t["metadata"])
		case "CronJob":
			t := object(object(object(spec["jobTemplate"])["spec"])["template"])
			p = object(t["spec"])
			pm = object(t["metadata"])
		}
		if p != nil {
			profile := "restricted"
			for _, s := range r.Services {
				if s.SecurityProfile == "baseline" && (len(r.Services) == 1 || s.Name == name || s.Name == text(object(pm["labels"])["app.kubernetes.io/name"])) {
					profile = "baseline"
				}
			}
			if text(object(pm["labels"])[TemplateLabel]) != r.ID {
				fail(where, ErrTemplateLabel)
			}
			checkPodSecurity(pm, p, profile, where, fail)
			checkPod(p, where, k != "Job" && k != "CronJob", fail)
		}
		if k == "Cluster" && api == "postgresql.cnpg.io/v1" {
			if !digestImage.MatchString(text(spec["imageName"])) {
				fail(where, "CNPG imageName must use sha256 digest")
			}
			checkResources(object(spec["resources"]), where, fail)
			if text(object(object(spec["inheritedMetadata"])["labels"])[TemplateLabel]) != r.ID {
				fail(where+"/inheritedMetadata", ErrTemplateLabel)
			}
			provider := false
			for _, ref := range r.Requires {
				if ref == "databases/cloudnative-pg" {
					provider = true
				}
			}
			if !provider {
				fail(where, ErrCNPGProvider)
			}
			if len(r.Security.Risks) == 0 {
				fail(where, "operator-managed workload must document generated pod and service-account trust")
			}
		}
		if strings.HasPrefix(api, "postgresql.cnpg.io/") || strings.HasPrefix(api, "barmancloud.cnpg.io/") {
			if !hasProvider(r, "databases/cloudnative-pg") {
				fail(where, "CNPG/Barman custom resources require databases/cloudnative-pg")
			}
		}
		if k == "Cluster" && api == "postgresql.cnpg.io/v1" {
			sec := text(object(spec["seccompProfile"])["type"])
			if sec != "RuntimeDefault" && sec != "Localhost" {
				fail(where, "CNPG restricted seccomp required")
			}
			for _, key := range []string{"postgresUID", "postgresGID"} {
				if v, ok := spec[key]; ok && fmt.Sprint(v) == "0" {
					fail(where, "CNPG root identity forbidden")
				}
			}
			for _, plugin := range array(spec["plugins"]) {
				if text(object(plugin)["name"]) != "barman-cloud.cloudnative-pg.io" {
					fail(where, "unsupported CNPG plugin forbidden")
				}
			}
		}
		if k == "ObjectStore" && api == "barmancloud.cnpg.io/v1" {
			checkResources(object(object(spec["instanceSidecarConfiguration"])["resources"]), where+"/barman-sidecar", fail)
		}
		if k == "PerconaServerMongoDB" && api == "psmdb.percona.com/v1" {
			checkPercona(r, d, docs, where, fail)
		}
		if k == "PerconaServerMongoDBBackup" && api == "psmdb.percona.com/v1" {
			checkPerconaBackup(r, d, docs, where, fail)
		}
		if k == "Dragonfly" && api == "dragonflydb.io/v1alpha1" {
			checkDragonfly(r, d, docs, where, fail)
		}
		if api == "k8s.mariadb.com/v1alpha1" {
			checkMariaDB(r, d, docs, namespace, where, fail)
		}
		if k == "Middleware" && api == "traefik.io/v1alpha1" {
			checkMiddleware(r, d, where, fail)
		}
		if k == "NetworkPolicy" {
			sel := object(spec["podSelector"])
			if !ownPods(sel, r.ID) {
				fail(where, ErrForeignPods, "podSelector")
			}
			if name == r.ID+"-default-deny" && len(sel) == 1 && len(object(sel["matchLabels"])) == 1 && len(array(spec["policyTypes"])) == 1 && contains(array(spec["policyTypes"]), "Ingress") && len(array(spec["ingress"])) == 0 {
				defaultDeny = true
			}
			checkPolicyRules(r, spec, namespace, where, fail)
		}
	}
	if !defaultDeny {
		fail("NetworkPolicy", ErrDefaultDeny)
	}
	return errors.Join(errs...)
}

// checkPolicyRules checks the peers of a NetworkPolicy. A rule with no peer or
// an empty peer allows all traffic. A pod selector for the template's own
// namespace selects only the template's pods.
func checkPolicyRules(r *Template, spec map[string]any, namespace, where string, fail func(string, Rule, ...string)) {
	for _, direction := range []string{"ingress", "egress"} {
		peer := "from"
		if direction == "egress" {
			peer = "to"
		}
		for _, rule := range array(spec[direction]) {
			peers := array(object(rule)[peer])
			if len(peers) == 0 {
				fail(where, "unbounded rule forbidden", direction)
			}
			for _, raw := range peers {
				v := object(raw)
				if len(v) == 0 {
					fail(where, "empty peer permits all traffic")
				}
				ns, hasNamespace := v["namespaceSelector"]
				sel, hasPods := v["podSelector"]
				own := !hasNamespace || text(object(object(ns)["matchLabels"])["kubernetes.io/metadata.name"]) == namespace
				if own && (hasNamespace || hasPods) && !ownPods(object(sel), r.ID) {
					fail(where, ErrForeignPods, direction)
				}
				ip := object(v["ipBlock"])
				if text(ip["cidr"]) == "0.0.0.0/0" || text(ip["cidr"]) == "::/0" {
					if len(r.Security.Risks) == 0 {
						fail(where, "public network access requires risk documentation")
					}
				}
			}
		}
	}
}

// hasKey reports whether a map anywhere in v has one of keys.
func hasKey(v any, keys ...string) bool {
	switch v := v.(type) {
	case map[string]any:
		for k, child := range v {
			if slices.Contains(keys, k) || hasKey(child, keys...) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if hasKey(child, keys...) {
				return true
			}
		}
	}
	return false
}

func checkResources(r map[string]any, w string, fail func(string, Rule, ...string)) {
	for _, level := range []string{"requests", "limits"} {
		m := object(r[level])
		for _, key := range []string{"cpu", "memory"} {
			v := fmt.Sprint(m[key])
			match := resourceQuantity.FindStringSubmatch(v)
			positive := false
			if match != nil {
				n, e := strconv.ParseFloat(match[1], 64)
				positive = e == nil && n > 0
			}
			if !positive {
				fail(w, ErrPositiveResource, level+"."+key)
			}
		}
	}
}

// checkPod checks what Pod Security Admission does not.
func checkPod(p map[string]any, w string, requireProbes bool, fail func(string, Rule, ...string)) {
	if p["automountServiceAccountToken"] != false {
		fail(w, ErrPodTokenAutomount)
	}
	for _, v := range array(p["volumes"]) {
		for _, source := range array(object(object(v)["projected"])["sources"]) {
			if _, ok := object(source)["serviceAccountToken"]; ok {
				fail(w, "projected service account tokens forbidden")
			}
		}
	}
	// Stricter than baseline, which also allows the kernel.shm_rmid_forced,
	// net.ipv4.ip_local_port_range and net.ipv4.tcp_* sysctls.
	for _, v := range array(object(p["securityContext"])["sysctls"]) {
		n := text(object(v)["name"])
		if n != "net.ipv4.ip_unprivileged_port_start" && n != "net.ipv4.ping_group_range" {
			fail(w, ErrUnsafeSysctl, n)
		}
	}
	for _, k := range []string{"containers", "initContainers", "ephemeralContainers"} {
		for _, v := range array(p[k]) {
			c := object(v)
			cw := w + "/" + text(c["name"])
			if !digestImage.MatchString(text(c["image"])) {
				fail(cw, ErrImageDigest)
			}
			for _, v := range array(c["volumeMounts"]) {
				if text(object(v)["mountPropagation"]) == "Bidirectional" {
					fail(cw, "bidirectional mount propagation forbidden")
				}
			}
			checkResources(object(c["resources"]), cw, fail)
			if k == "containers" && requireProbes {
				if len(object(c["readinessProbe"])) == 0 || len(object(c["livenessProbe"])) == 0 {
					fail(cw, ErrProbesRequired)
				}
			}
		}
	}
}
