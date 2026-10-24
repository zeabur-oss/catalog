package templates

import (
	"errors"
	"fmt"
	"regexp"
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

// Safety checks the rendered objects themselves.
func Safety(r *Template, b []byte, namespace string) error {
	docs, err := DecodeDocuments(b)
	if err != nil {
		return err
	}
	var errs []error
	defaultDeny := false
	var policies []map[string]any
	fail := func(where, msg string) { errs = append(errs, fmt.Errorf("%s: %s", where, msg)) }
	seen := map[string]bool{}
	maintenance := checkMaintenance(r, docs, namespace, fail)
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
			fail(where, "resource escapes target namespace")
		}
		switch k {
		case "Namespace", "Node", "PersistentVolume", "StorageClass", "CustomResourceDefinition", "ClusterRole", "ClusterRoleBinding", "MutatingWebhookConfiguration", "ValidatingWebhookConfiguration", "APIService", "PriorityClass", "CertificateSigningRequest":
			fail(where, "cluster-scoped resources and RBAC are forbidden")
		}
		switch api + "/" + k {
		case "v1/Pod", "v1/Service", "v1/ConfigMap", "v1/Secret", "v1/ServiceAccount", "v1/PersistentVolumeClaim", "apps/v1/Deployment", "apps/v1/StatefulSet", "apps/v1/DaemonSet", "apps/v1/ReplicaSet", "batch/v1/Job", "batch/v1/CronJob", "networking.k8s.io/v1/Ingress", "networking.k8s.io/v1/NetworkPolicy", "policy/v1/PodDisruptionBudget", "autoscaling/v2/HorizontalPodAutoscaler", "cert-manager.io/v1/Certificate", "postgresql.cnpg.io/v1/Cluster", "postgresql.cnpg.io/v1/Backup", "postgresql.cnpg.io/v1/ScheduledBackup", "barmancloud.cnpg.io/v1/ObjectStore", "psmdb.percona.com/v1/PerconaServerMongoDB", "traefik.io/v1alpha1/Middleware":
		case "psmdb.percona.com/v1/PerconaServerMongoDBBackup":
		case "dragonflydb.io/v1alpha1/Dragonfly":
		case "k8s.mariadb.com/v1alpha1/MariaDB", "k8s.mariadb.com/v1alpha1/Backup", "k8s.mariadb.com/v1alpha1/Database", "k8s.mariadb.com/v1alpha1/User", "k8s.mariadb.com/v1alpha1/Grant":
		case "rbac.authorization.k8s.io/v1/Role", "rbac.authorization.k8s.io/v1/RoleBinding":
		default:
			fail(where, "unsupported resource kind; the linter needs a namespaced safety adapter for it")
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
				fail(where, "Traefik Ingress requires exact ingress/traefik provider")
			}
			if text(object(meta["annotations"])["cert-manager.io/cluster-issuer"]) != "" && !hasProvider(r, "certificates/cert-manager") {
				fail(where, "ClusterIssuer annotation requires exact certificates/cert-manager provider")
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
			for key, v := range object(pm["annotations"]) {
				if (strings.Contains(key, "apparmor") || strings.Contains(key, "seccomp")) && strings.Contains(strings.ToLower(text(v)), "unconfined") {
					fail(where, "unconfined legacy security annotation forbidden")
				}
			}
			checkPod(p, profile, where, k != "Job" && k != "CronJob", maintenance[where], fail)
		}
		if k == "Cluster" && api == "postgresql.cnpg.io/v1" {
			if !digestImage.MatchString(text(spec["imageName"])) {
				fail(where, "CNPG imageName must use sha256 digest")
			}
			checkResources(object(spec["resources"]), where, fail)
			provider := false
			for _, ref := range r.Requires {
				if ref == "databases/cloudnative-pg" {
					provider = true
				}
			}
			if !provider {
				fail(where, "CNPG workload requires exact databases/cloudnative-pg provider")
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
			policies = append(policies, spec)
			sel := object(spec["podSelector"])
			if len(sel) == 0 && contains(array(spec["policyTypes"]), "Ingress") && contains(array(spec["policyTypes"]), "Egress") && len(array(spec["ingress"])) == 0 && len(array(spec["egress"])) == 0 {
				defaultDeny = true
			}
		}
	}
	if !defaultDeny {
		fail("NetworkPolicy", "an all-pods default deny for both ingress and egress is required")
	}
	// A policy with empty rules is safe; additive allow rules must not accidentally select everything.
	for _, p := range policies {
		for _, direction := range []string{"ingress", "egress"} {
			for _, rule := range array(p[direction]) {
				m := object(rule)
				peer := "from"
				if direction == "egress" {
					peer = "to"
				}
				if len(array(m[peer])) == 0 {
					fail("NetworkPolicy", "unbounded "+direction+" rule is forbidden")
				}
				for _, raw := range array(m[peer]) {
					v := object(raw)
					if len(v) == 0 {
						fail("NetworkPolicy", "empty peer permits all traffic")
					}
					ip := object(v["ipBlock"])
					if text(ip["cidr"]) == "0.0.0.0/0" || text(ip["cidr"]) == "::/0" {
						if len(r.Security.Egress) == 0 || len(r.Security.Risks) == 0 {
							fail("NetworkPolicy", "public network access requires explicit egress and risk documentation")
						}
					}
				}
			}
		}
	}
	return errors.Join(errs...)
}
func checkResources(r map[string]any, w string, fail func(string, string)) {
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
				fail(w, "positive resource "+level+"."+key+" required")
			}
		}
	}
}
func checkPod(p map[string]any, profile, w string, requireProbes, allowMaintenanceToken bool, fail func(string, string)) {
	if p["automountServiceAccountToken"] != false {
		fail(w, "automountServiceAccountToken must explicitly be false")
	}
	for _, k := range []string{"hostNetwork", "hostPID", "hostIPC"} {
		if truth(p[k]) {
			fail(w, k+" forbidden")
		}
	}
	for _, v := range array(p["volumes"]) {
		m := object(v)
		if _, ok := m["hostPath"]; ok {
			fail(w, "hostPath volume forbidden")
		}
		if profile == "restricted" {
			for typ := range m {
				switch typ {
				case "name", "configMap", "downwardAPI", "emptyDir", "ephemeral", "persistentVolumeClaim", "projected", "secret":
				default:
					fail(w, "volume type not permitted by restricted profile: "+typ)
				}
			}
		}
		for _, source := range array(object(m["projected"])["sources"]) {
			if _, ok := object(source)["serviceAccountToken"]; ok && !allowMaintenanceToken {
				fail(w, "projected service account tokens forbidden")
			}
		}
	}
	psc := object(p["securityContext"])
	for _, v := range array(psc["sysctls"]) {
		n := text(object(v)["name"])
		if n != "net.ipv4.ip_unprivileged_port_start" && n != "net.ipv4.ping_group_range" {
			fail(w, "sysctl not in safe allowlist: "+n)
		}
	}
	for _, k := range []string{"containers", "initContainers", "ephemeralContainers"} {
		for _, v := range array(p[k]) {
			c := object(v)
			cw := w + "/" + text(c["name"])
			sc := object(c["securityContext"])
			if !digestImage.MatchString(text(c["image"])) {
				fail(cw, "image must use immutable sha256 digest")
			}
			if truth(sc["privileged"]) {
				fail(cw, "privileged container forbidden")
			}
			if truth(sc["procMount"]) || text(sc["procMount"]) == "Unmasked" {
				fail(cw, "unmasked proc mount forbidden")
			}
			for _, port := range array(c["ports"]) {
				if n := object(port)["hostPort"]; n != nil && fmt.Sprint(n) != "0" {
					fail(cw, "hostPort forbidden")
				}
			}
			for _, probe := range []string{"readinessProbe", "livenessProbe", "startupProbe"} {
				pr := object(c[probe])
				if text(object(pr["httpGet"])["host"]) != "" || text(object(pr["tcpSocket"])["host"]) != "" {
					fail(cw, "host probe override forbidden")
				}
			}
			for _, hook := range []string{"postStart", "preStop"} {
				handler := object(object(c["lifecycle"])[hook])
				if text(object(handler["httpGet"])["host"]) != "" || text(object(handler["tcpSocket"])["host"]) != "" {
					fail(cw, "host lifecycle hook override forbidden")
				}
			}
			for _, v := range array(c["volumeMounts"]) {
				if text(object(v)["mountPropagation"]) == "Bidirectional" {
					fail(cw, "bidirectional mount propagation forbidden")
				}
			}
			caps := object(sc["capabilities"])
			for _, cap := range array(caps["add"]) {
				s := text(cap)
				if profile == "restricted" && s != "NET_BIND_SERVICE" {
					fail(cw, "restricted capability forbidden: "+s)
				}
				if profile == "baseline" && !strings.Contains(" CHOWN DAC_OVERRIDE FOWNER FSETID KILL MKNOD NET_BIND_SERVICE SETFCAP SETGID SETPCAP SETUID SYS_CHROOT AUDIT_WRITE ", " "+s+" ") {
					fail(cw, "baseline capability forbidden: "+s)
				}
			}
			for _, ctx := range []map[string]any{psc, sc} {
				if v := object(ctx["seLinuxOptions"]); len(v) > 0 {
					typ := text(v["type"])
					if typ != "" && typ != "container_t" && typ != "container_init_t" && typ != "container_kvm_t" && typ != "container_engine_t" {
						fail(cw, "forbidden SELinux type")
					}
					if text(v["user"]) != "" || text(v["role"]) != "" {
						fail(cw, "SELinux user/role overrides forbidden")
					}
				}
				a := object(ctx["appArmorProfile"])
				if text(a["type"]) == "Unconfined" {
					fail(cw, "unconfined AppArmor forbidden")
				}
				if text(object(ctx["seccompProfile"])["type"]) == "Unconfined" {
					fail(cw, "unconfined seccomp forbidden")
				}
				if truth(object(ctx["windowsOptions"])["hostProcess"]) {
					fail(cw, "Windows hostProcess forbidden")
				}
			}
			if profile == "restricted" {
				if sc["allowPrivilegeEscalation"] != false {
					fail(cw, "allowPrivilegeEscalation must be false")
				}
				if !contains(array(caps["drop"]), "ALL") {
					fail(cw, "capabilities.drop must include ALL")
				}
				nonroot, explicit := sc["runAsNonRoot"]
				if !explicit {
					nonroot = psc["runAsNonRoot"]
				}
				if nonroot != true {
					fail(cw, "effective runAsNonRoot must be true")
				}
				for _, ctx := range []map[string]any{psc, sc} {
					if uid, ok := ctx["runAsUser"]; ok && fmt.Sprint(uid) == "0" {
						fail(cw, "runAsUser 0 forbidden")
					}
				}
				sec := text(object(sc["seccompProfile"])["type"])
				if sec == "" {
					sec = text(object(psc["seccompProfile"])["type"])
				}
				if sec != "RuntimeDefault" && sec != "Localhost" {
					fail(cw, "restricted seccomp must be RuntimeDefault or Localhost")
				}
			}
			checkResources(object(c["resources"]), cw, fail)
			if k == "containers" && requireProbes {
				if len(object(c["readinessProbe"])) == 0 || len(object(c["livenessProbe"])) == 0 {
					fail(cw, "readiness and liveness probes required")
				}
			}
		}
	}
}
