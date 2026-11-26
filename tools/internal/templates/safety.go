package templates

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zeabur-oss/catalog/tools/internal/catalog"
	"github.com/zeabur-oss/catalog/tools/internal/kubeobj"
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

// Rule is a safety rule that a rendered object breaks. Safety wraps each one it finds.
type Rule string

func (r Rule) Error() string { return string(r) }

// Rules that callers can match with errors.Is.
const (
	ErrNamespaceEscape       Rule = "resource escapes target namespace"
	ErrClusterScoped         Rule = "cluster-scoped resources and RBAC are forbidden"
	ErrTraefikProvider       Rule = "Traefik Ingress requires exact traefik provider"
	ErrClusterIssuerProvider Rule = "ClusterIssuer annotation requires exact cert-manager provider"
	ErrCNPGProvider          Rule = "CNPG workload requires exact cloudnative-pg provider"
	ErrCNPGResourceProvider  Rule = "CNPG/Barman custom resources require cloudnative-pg"
	ErrPerconaProvider       Rule = "Percona operand requires exact percona-mongodb"
	ErrMiddlewareProvider    Rule = "Middleware requires traefik"
	ErrIngressPolicy         Rule = "a service listed in security.unauthenticated needs an ingress NetworkPolicy whose podSelector names it"
	ErrUnknownService        Rule = "security.unauthenticated names a service the template does not declare"
	ErrTemplateLabel         Rule = "pods must have the label " + TemplateLabel + " set to the template ID"
	ErrForeignPods           Rule = "NetworkPolicy selectors must select the template's own pods, or in a peer its clients, by label"
	ErrPositiveResource      Rule = "positive resource required"
	ErrPodTokenAutomount     Rule = "automountServiceAccountToken must explicitly be false"
	ErrPodSecurity           Rule = "pod breaks the Pod Security Standard of its service"
	ErrUnsafeSysctl          Rule = "sysctl not in safe allowlist"
	ErrImageDigest           Rule = "image must use immutable sha256 digest"
	ErrProbesRequired        Rule = "readiness and liveness probes required"
	ErrIPAllowListOnly       Rule = "only explicit ipAllowList Middleware supported"
	ErrIPAllowListCIDR       Rule = "ipAllowList requires non-public, valid explicit CIDR"
	ErrUnsupportedOverride   Rule = "unsupported override forbidden"
	ErrDragonflyProvider     Rule = "Dragonfly requires exact dragonfly-operator"
	ErrDragonflyDigest       Rule = "Dragonfly image must use sha256 digest"
	ErrDragonflyPassword     Rule = "Dragonfly authentication.passwordFromSecret required"
	ErrDragonflyFlag         Rule = "unauthenticated Dragonfly flag forbidden"
	ErrMariaDBRefNamespace   Rule = "mariaDbRef escapes target namespace"
	ErrMariaDBRefUndeclared  Rule = "mariaDbRef must name a MariaDB declared in this template"
	ErrMariaDBProvider       Rule = "MariaDB Operator resources require exact mariadb-operator"
	ErrMOCOProvider          Rule = "MySQLCluster requires exact moco"
	ErrMariaDBBackupS3       Rule = "backup must use S3 with explicit credentials and TLS verification"
	ErrGlobalGrant           Rule = "Grant must name one database; global privileges forbidden"
	ErrPerconaBackupProvider Rule = "Percona backup requires exact percona-mongodb"
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

// ownPods reports whether the selector selects only the pods of template id.
func ownPods(sel *metav1.LabelSelector, id string) bool {
	return id != "" && sel != nil && sel.MatchLabels[TemplateLabel] == id
}

// clientLabel is the label that lets pods of other templates reach a template's
// unauthenticated services, set to "true", such as templates.zeabur.com/ollama-client.
func clientLabel(id string) string {
	return "templates.zeabur.com/" + id + "-client"
}

// clientPods reports whether sel selects the pods that carry the client label of template id.
func clientPods(sel *metav1.LabelSelector, id string) bool {
	return id != "" && sel != nil && sel.MatchLabels[clientLabel(id)] == "true"
}

// coreKinds are the kinds that Kubernetes and cert-manager own, which a template may render.
var coreKinds = map[string]bool{
	"v1/Pod": true, "v1/Service": true, "v1/ConfigMap": true, "v1/Secret": true, "v1/ServiceAccount": true, "v1/PersistentVolumeClaim": true,
	"apps/v1/Deployment": true, "apps/v1/StatefulSet": true, "apps/v1/DaemonSet": true, "apps/v1/ReplicaSet": true,
	"batch/v1/Job": true, "batch/v1/CronJob": true,
	"networking.k8s.io/v1/Ingress": true, "networking.k8s.io/v1/NetworkPolicy": true,
	"policy/v1/PodDisruptionBudget": true, "autoscaling/v2/HorizontalPodAutoscaler": true, "cert-manager.io/v1/Certificate": true,
}

// Safety checks the rendered objects themselves.
func Safety(r *Template, b []byte, namespace string) error {
	docs, err := DecodeDocuments(b)
	if err != nil {
		return err
	}
	var errs []error
	var ingressPolicies []map[string]string
	fail := func(where string, rule Rule, detail ...string) {
		err := fmt.Errorf("%s: %w", where, rule)
		if len(detail) > 0 {
			err = fmt.Errorf("%w: %s", err, strings.Join(detail, " "))
		}
		errs = append(errs, err)
	}
	seen := map[string]bool{}
	for _, d := range docs {
		k := d.GetKind()
		api := d.GetAPIVersion()
		name := d.GetName()
		where := k + "/" + name
		key := api + "/" + where
		if seen[key] {
			fail(where, "duplicate resource")
		}
		seen[key] = true
		if ns := d.GetNamespace(); ns != "" && ns != namespace {
			fail(where, ErrNamespaceEscape)
		}
		switch k {
		case "Namespace", "Node", "PersistentVolume", "StorageClass", "CustomResourceDefinition", "ClusterRole", "ClusterRoleBinding", "MutatingWebhookConfiguration", "ValidatingWebhookConfiguration", "APIService", "PriorityClass", "CertificateSigningRequest":
			fail(where, ErrClusterScoped)
		}
		if !coreKinds[api+"/"+k] && !operatorKind(api, k) {
			fail(where, ErrUnsupportedKind)
		}
		if hasKey(d.Object, "storageClassName", "storageClass") {
			fail(where, ErrStorageClass)
		}
		if k == "Secret" {
			fail(where, "rendered Secrets forbidden; use explicit inputs and preexisting Secrets")
		}
		// Custom resources stay maps; core kinds are read as their Kubernetes types.
		switch api + "/" + k {
		case "v1/ServiceAccount":
			var sa corev1.ServiceAccount
			if err := kubeobj.Typed(d, &sa); err != nil {
				fail(where, "invalid ServiceAccount", err.Error())
			} else if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
				fail(where, "ServiceAccount must disable token automount")
			}
		case "v1/Service":
			var svc corev1.Service
			if err := kubeobj.Typed(d, &svc); err != nil {
				fail(where, "invalid Service", err.Error())
			} else if t := svc.Spec.Type; t == corev1.ServiceTypeNodePort || t == corev1.ServiceTypeLoadBalancer || len(svc.Spec.ExternalIPs) > 0 {
				fail(where, "externally exposed Service forbidden; use ingress")
			}
		case "networking.k8s.io/v1/Ingress":
			var ing networkingv1.Ingress
			if err := kubeobj.Typed(d, &ing); err != nil {
				fail(where, "invalid Ingress", err.Error())
				break
			}
			if c := ing.Spec.IngressClassName; c != nil && *c == "traefik" && !hasProvider(r, "traefik") {
				fail(where, ErrTraefikProvider)
			}
			if ing.Annotations["cert-manager.io/cluster-issuer"] != "" && !hasProvider(r, "cert-manager") {
				fail(where, ErrClusterIssuerProvider)
			}
		}
		tmpl, isPod, err := kubeobj.PodTemplate(d)
		if err != nil {
			fail(where, "invalid pod", err.Error())
		}
		if isPod {
			profile := "restricted"
			for _, s := range r.Services {
				if s.PodSecurity == "baseline" && (len(r.Services) == 1 || s.Name == name || s.Name == tmpl.Labels["app.kubernetes.io/name"]) {
					profile = "baseline"
				}
			}
			if tmpl.Labels[TemplateLabel] != r.ID {
				fail(where, ErrTemplateLabel)
			}
			checkPodSecurity(tmpl.ObjectMeta, tmpl.Spec, profile, where, fail)
			checkPod(&tmpl.Spec, where, k != "Job" && k != "CronJob", fail)
		}
		checkKinds(kindContext{r: r, d: d.Object, docs: docs, namespace: namespace, where: where, fail: fail})
		if api == "networking.k8s.io/v1" && k == "NetworkPolicy" {
			var np networkingv1.NetworkPolicy
			if err := kubeobj.Typed(d, &np); err != nil {
				fail(where, "invalid NetworkPolicy", err.Error())
				continue
			}
			if !ownPods(&np.Spec.PodSelector, r.ID) {
				fail(where, ErrForeignPods, "podSelector")
			}
			if slices.Contains(np.Spec.PolicyTypes, networkingv1.PolicyTypeIngress) {
				ingressPolicies = append(ingressPolicies, np.Spec.PodSelector.MatchLabels)
			}
			checkPolicyRules(r, &np, namespace, where, fail)
		}
	}
	for _, name := range r.Security.Unauthenticated {
		if !slices.ContainsFunc(r.Services, func(s Service) bool { return s.Name == name }) {
			fail("security.unauthenticated", ErrUnknownService, name)
			continue
		}
		named := slices.ContainsFunc(ingressPolicies, func(labels map[string]string) bool {
			for k, v := range labels {
				if k != TemplateLabel && v == name {
					return true
				}
			}
			return false
		})
		if !named {
			fail("NetworkPolicy", ErrIngressPolicy, name)
		}
	}
	return errors.Join(errs...)
}

// checkPolicyRules checks the peers of a NetworkPolicy. A rule with no peer or
// an empty peer allows all traffic. A pod selector for the template's own
// namespace selects only the template's pods or its clients.
func checkPolicyRules(r *Template, np *networkingv1.NetworkPolicy, namespace, where string, fail func(string, Rule, ...string)) {
	check := func(direction string, peers []networkingv1.NetworkPolicyPeer) {
		if len(peers) == 0 {
			fail(where, "unbounded rule forbidden", direction)
		}
		for _, v := range peers {
			if v.NamespaceSelector == nil && v.PodSelector == nil && v.IPBlock == nil {
				fail(where, "empty peer permits all traffic")
			}
			own := v.NamespaceSelector == nil || v.NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] == namespace
			if own && (v.NamespaceSelector != nil || v.PodSelector != nil) && !ownPods(v.PodSelector, r.ID) && !clientPods(v.PodSelector, r.ID) {
				fail(where, ErrForeignPods, direction)
			}
			if v.IPBlock != nil && (v.IPBlock.CIDR == "0.0.0.0/0" || v.IPBlock.CIDR == "::/0") && len(r.Security.Risks) == 0 {
				fail(where, "public network access requires risk documentation")
			}
		}
	}
	for _, rule := range np.Spec.Ingress {
		check("ingress", rule.From)
	}
	for _, rule := range np.Spec.Egress {
		check("egress", rule.To)
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

// checkResources checks the resources of a custom resource, which are a map in its object.
func checkResources(r map[string]any, w string, fail func(string, Rule, ...string)) {
	var req corev1.ResourceRequirements
	if err := convert(r, &req); err != nil {
		fail(w, ErrPositiveResource, err.Error())
		return
	}
	checkRequirements(req, w, fail)
}

// checkRequirements fails unless the CPU and memory requests and limits are above zero.
func checkRequirements(r corev1.ResourceRequirements, w string, fail func(string, Rule, ...string)) {
	for _, level := range []struct {
		name string
		list corev1.ResourceList
	}{{"requests", r.Requests}, {"limits", r.Limits}} {
		for _, key := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
			if q, ok := level.list[key]; !ok || q.Sign() <= 0 {
				fail(w, ErrPositiveResource, level.name+"."+string(key))
			}
		}
	}
}

// checkPod checks what Pod Security Admission does not.
func checkPod(p *corev1.PodSpec, w string, requireProbes bool, fail func(string, Rule, ...string)) {
	if p.AutomountServiceAccountToken == nil || *p.AutomountServiceAccountToken {
		fail(w, ErrPodTokenAutomount)
	}
	for _, v := range p.Volumes {
		if v.Projected == nil {
			continue
		}
		for _, source := range v.Projected.Sources {
			if source.ServiceAccountToken != nil {
				fail(w, "projected service account tokens forbidden")
			}
		}
	}
	// Stricter than baseline, which also allows the kernel.shm_rmid_forced,
	// net.ipv4.ip_local_port_range and net.ipv4.tcp_* sysctls.
	if p.SecurityContext != nil {
		for _, s := range p.SecurityContext.Sysctls {
			if s.Name != "net.ipv4.ip_unprivileged_port_start" && s.Name != "net.ipv4.ping_group_range" {
				fail(w, ErrUnsafeSysctl, s.Name)
			}
		}
	}
	for _, c := range kubeobj.Containers(p) {
		cw := w + "/" + c.Name
		if !catalog.PinnedImage.MatchString(c.Image) {
			fail(cw, ErrImageDigest)
		}
		for _, m := range c.VolumeMounts {
			if m.MountPropagation != nil && *m.MountPropagation == corev1.MountPropagationBidirectional {
				fail(cw, "bidirectional mount propagation forbidden")
			}
		}
		checkRequirements(c.Resources, cw, fail)
		if c.List == "containers" && requireProbes && (c.ReadinessProbe == nil || c.LivenessProbe == nil) {
			fail(cw, ErrProbesRequired)
		}
	}
}
