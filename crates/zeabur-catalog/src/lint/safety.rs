//! The rules for every rendered object: where it goes, what kind it is, and for a pod, a
//! Service, an Ingress or a NetworkPolicy, what it may do.

use std::collections::{BTreeMap, BTreeSet};

use k8s_openapi::api::core::v1::{
    Container, PodSpec, ResourceRequirements, Service, ServiceAccount,
};
use k8s_openapi::api::networking::v1::{Ingress, NetworkPolicy, NetworkPolicyPeer};
use k8s_openapi::apimachinery::pkg::apis::meta::v1::{LabelSelector, ObjectMeta};
use serde::Deserialize;
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
use serde_json::Value;
use zeabur_catalog_engine::pod_security;
use zeabur_catalog_engine::quantity::Quantity;
use zeabur_catalog_engine::template::TEMPLATE_LABEL;
use zeabur_catalog_format::common::PodSecurity;
use zeabur_catalog_format::problem::LintRule;

use super::view::{Head, has_key, view};
use super::{Found, Subject, operators};
use crate::render::pinned;

/// The kinds of Kubernetes and cert-manager that a template may render, by API version.
const CORE_KINDS: [(&str, &str); 17] = [
    ("v1", "Pod"),
    ("v1", "Service"),
    ("v1", "ConfigMap"),
    ("v1", "Secret"),
    ("v1", "ServiceAccount"),
    ("v1", "PersistentVolumeClaim"),
    ("apps/v1", "Deployment"),
    ("apps/v1", "StatefulSet"),
    ("apps/v1", "DaemonSet"),
    ("apps/v1", "ReplicaSet"),
    ("batch/v1", "Job"),
    ("batch/v1", "CronJob"),
    ("networking.k8s.io/v1", "Ingress"),
    ("networking.k8s.io/v1", "NetworkPolicy"),
    ("policy/v1", "PodDisruptionBudget"),
    ("autoscaling/v2", "HorizontalPodAutoscaler"),
    ("cert-manager.io/v1", "Certificate"),
];

/// Kinds that are cluster-scoped or grant rights, of any API version.
const CLUSTER_SCOPED: [&str; 12] = [
    "Namespace",
    "Node",
    "PersistentVolume",
    "StorageClass",
    "CustomResourceDefinition",
    "ClusterRole",
    "ClusterRoleBinding",
    "MutatingWebhookConfiguration",
    "ValidatingWebhookConfiguration",
    "APIService",
    "PriorityClass",
    "CertificateSigningRequest",
];

/// The sysctls a pod may set. Stricter than baseline, which also allows
/// `kernel.shm_rmid_forced`, `net.ipv4.ip_local_port_range` and `net.ipv4.tcp_*`.
const SAFE_SYSCTLS: [&str; 2] = [
    "net.ipv4.ip_unprivileged_port_start",
    "net.ipv4.ping_group_range",
];

/// Adds what `docs` break of the rules for every object, the operator rules included.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
pub(super) fn check(subject: &Subject<'_>, docs: &[Value], found: &mut Found) {
    let mut seen = BTreeSet::new();
    let mut ingress_policies = Vec::new();
    for doc in docs {
        let head: Head = view(doc);
        let (api, kind, name) = (
            head.api_version.0.as_str(),
            head.kind.0.as_str(),
            head.name(),
        );
        let at = &format!("{kind}/{name}");
        if !seen.insert(format!("{api}/{at}")) {
            found.push(at, LintRule::DuplicateResource);
        }
        if let Some(namespace) = head.metadata.0.namespace.as_deref()
            && !namespace.is_empty()
            && namespace != subject.namespace
        {
            found.push(at, LintRule::NamespaceEscape);
        }
        if CLUSTER_SCOPED.contains(&kind) {
            found.push(at, LintRule::ClusterScoped);
        }
        if !CORE_KINDS.contains(&(api, kind)) && !operators::covers(api, kind) {
            found.push(at, LintRule::UnsupportedKind);
        }
        if has_key(doc, &["storageClassName", "storageClass"]) {
            found.push(at, LintRule::StorageClass);
        }
        if kind == "Secret" {
            found.push(at, LintRule::RenderedSecret);
        }
        typed(subject, (api, kind), doc, at, found);
        match pod_security::pod_template(doc) {
            Err(e) => found.push(at, invalid("pod", &e)),
            Ok(None) => {}
            Ok(Some(template)) => {
                let meta = template.metadata.unwrap_or_default();
                let spec = template.spec.unwrap_or_default();
                let label = |key: &str| {
                    meta.labels
                        .as_ref()
                        .and_then(|l| l.get(key))
                        .map_or("", String::as_str)
                };
                let services = &subject.template.services;
                let app = label("app.kubernetes.io/name");
                let level = if services.iter().any(|s| {
                    s.pod_security == PodSecurity::Baseline
                        && (services.len() == 1 || s.name == name || s.name == app)
                }) {
                    PodSecurity::Baseline
                } else {
                    PodSecurity::Restricted
                };
                if label(TEMPLATE_LABEL) != subject.id {
                    found.push(at, LintRule::TemplateLabel);
                }
                pod_security(&meta, &spec, level, at, found);
                check_pod(&spec, at, kind != "Job" && kind != "CronJob", found);
            }
        }
        operators::check(subject, doc, &head, docs, at, found);
        if (api, kind) == ("networking.k8s.io/v1", "NetworkPolicy") {
            network_policy(subject, doc, at, &mut ingress_policies, found);
        }
    }
    unauthenticated(subject, &ingress_policies, found);
}

/// The rule that `doc`, a `kind`, breaks when it does not read as one.
fn invalid(kind: &str, e: &serde_json::Error) -> LintRule {
    LintRule::Invalid {
        kind: kind.to_owned(),
        message: e.to_string(),
    }
}

/// Adds what a ServiceAccount, Service or Ingress breaks.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
fn typed(subject: &Subject<'_>, kind: (&str, &str), doc: &Value, at: &str, found: &mut Found) {
    match kind {
        ("v1", "ServiceAccount") => match ServiceAccount::deserialize(doc) {
            Err(e) => found.push(at, invalid("ServiceAccount", &e)),
            Ok(account) if account.automount_service_account_token != Some(false) => {
                found.push(at, LintRule::ServiceAccountToken);
            }
            Ok(_) => {}
        },
        ("v1", "Service") => match Service::deserialize(doc) {
            Err(e) => found.push(at, invalid("Service", &e)),
            Ok(service) => {
                let spec = service.spec.unwrap_or_default();
                if matches!(spec.type_.as_deref(), Some("NodePort" | "LoadBalancer"))
                    || spec.external_ips.is_some_and(|ips| !ips.is_empty())
                {
                    found.push(at, LintRule::ExposedService);
                }
            }
        },
        ("networking.k8s.io/v1", "Ingress") => match Ingress::deserialize(doc) {
            Err(e) => found.push(at, invalid("Ingress", &e)),
            Ok(ingress) => {
                let class = ingress.spec.and_then(|s| s.ingress_class_name);
                if class.as_deref() == Some("traefik") && !subject.requires("traefik") {
                    found.push(at, LintRule::TraefikProvider);
                }
                let issuer = ingress
                    .metadata
                    .annotations
                    .as_ref()
                    .and_then(|a| a.get("cert-manager.io/cluster-issuer"))
                    .is_some_and(|i| !i.is_empty());
                if issuer && !subject.requires("cert-manager") {
                    found.push(at, LintRule::ClusterIssuerProvider);
                }
            }
        },
        _ => {}
    }
}

/// Adds what a NetworkPolicy breaks, and the pod labels it selects when it governs ingress.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
fn network_policy(
    subject: &Subject<'_>,
    doc: &Value,
    at: &str,
    ingress_policies: &mut Vec<BTreeMap<String, String>>,
    found: &mut Found,
) {
    let policy = match NetworkPolicy::deserialize(doc) {
        Ok(policy) => policy,
        Err(e) => {
            found.push(at, invalid("NetworkPolicy", &e));
            return;
        }
    };
    let spec = policy.spec.unwrap_or_default();
    if !own_pods(spec.pod_selector.as_ref(), subject.id) {
        let selector = "podSelector".to_owned();
        found.push(at, LintRule::ForeignPods { selector });
    }
    if spec
        .policy_types
        .as_ref()
        .is_some_and(|t| t.iter().any(|t| t == "Ingress"))
    {
        let labels = spec
            .pod_selector
            .as_ref()
            .and_then(|s| s.match_labels.clone());
        ingress_policies.push(labels.unwrap_or_default());
    }
    let ingress = spec.ingress.iter().flatten().map(|r| ("ingress", &r.from));
    let egress = spec.egress.iter().flatten().map(|r| ("egress", &r.to));
    for (direction, peers) in ingress.chain(egress) {
        check_peers(
            subject,
            direction,
            peers.as_deref().unwrap_or_default(),
            at,
            found,
        );
    }
}

/// Adds a [`LintRule::PodSecurity`] when the pod breaks `level`.
pub(super) fn pod_security(
    meta: &ObjectMeta,
    spec: &PodSpec,
    level: PodSecurity,
    at: &str,
    found: &mut Found,
) {
    if let Some(refusal) = pod_security::check(level, meta, spec) {
        let refusal = refusal.to_string();
        let level = Some(level);
        found.push(at, LintRule::PodSecurity { level, refusal });
    }
}

/// Each service of `security.unauthenticated` is declared, and an ingress NetworkPolicy selects
/// it by a label other than the template label.
fn unauthenticated(
    subject: &Subject<'_>,
    ingress_policies: &[BTreeMap<String, String>],
    found: &mut Found,
) {
    let template = subject.template;
    for name in &template.security.unauthenticated {
        let service = name.clone();
        if !template.services.iter().any(|s| s.name == *name) {
            found.push(
                "security.unauthenticated",
                LintRule::UnknownService { service },
            );
            continue;
        }
        let named = ingress_policies
            .iter()
            .any(|labels| labels.iter().any(|(k, v)| k != TEMPLATE_LABEL && v == name));
        if !named {
            found.push("NetworkPolicy", LintRule::IngressPolicy { service });
        }
    }
}

/// Whether `selector` selects only the pods of template `id`.
fn own_pods(selector: Option<&LabelSelector>, id: &str) -> bool {
    matches_label(selector, TEMPLATE_LABEL, id)
}

/// Whether `selector` selects the pods with the client label of template `id`, such as
/// `templates.zeabur.com/ollama-client: "true"`, which may reach its unauthenticated services.
fn client_pods(selector: Option<&LabelSelector>, id: &str) -> bool {
    matches_label(
        selector,
        &format!("templates.zeabur.com/{id}-client"),
        "true",
    )
}

fn matches_label(selector: Option<&LabelSelector>, key: &str, value: &str) -> bool {
    !value.is_empty()
        && selector
            .and_then(|s| s.match_labels.as_ref())
            .and_then(|l| l.get(key))
            .is_some_and(|v| v == value)
}

/// Checks the peers of a NetworkPolicy rule. A rule without peers, or an empty peer, allows all
/// traffic. A pod selector for the template's own namespace selects only the template's pods or
/// its clients.
fn check_peers(
    subject: &Subject<'_>,
    direction: &str,
    peers: &[NetworkPolicyPeer],
    at: &str,
    found: &mut Found,
) {
    if peers.is_empty() {
        let direction = direction.to_owned();
        found.push(at, LintRule::UnboundedRule { direction });
    }
    for peer in peers {
        let namespaces = peer.namespace_selector.as_ref();
        let pods = peer.pod_selector.as_ref();
        if namespaces.is_none() && pods.is_none() && peer.ip_block.is_none() {
            found.push(at, LintRule::EmptyPeer);
        }
        let own = namespaces.is_none()
            || matches_label(namespaces, "kubernetes.io/metadata.name", subject.namespace);
        if own
            && (namespaces.is_some() || pods.is_some())
            && !own_pods(pods, subject.id)
            && !client_pods(pods, subject.id)
        {
            let selector = direction.to_owned();
            found.push(at, LintRule::ForeignPods { selector });
        }
        if peer
            .ip_block
            .as_ref()
            .is_some_and(|b| b.cidr == "0.0.0.0/0" || b.cidr == "::/0")
            && !subject.has_risks()
        {
            found.push(at, LintRule::PublicNetworkWithoutRisk);
        }
    }
}

/// Checks what Pod Security Admission does not: the token, sysctls, images, mounts, resources
/// and, for a long-running pod, probes.
fn check_pod(spec: &PodSpec, at: &str, probes: bool, found: &mut Found) {
    if spec.automount_service_account_token != Some(false) {
        found.push(at, LintRule::PodTokenAutomount);
    }
    let projected = spec
        .volumes
        .iter()
        .flatten()
        .filter_map(|v| v.projected.as_ref())
        .flat_map(|p| p.sources.iter().flatten());
    for source in projected {
        if source.service_account_token.is_some() {
            found.push(at, LintRule::ProjectedToken);
        }
    }
    let sysctls = spec
        .security_context
        .iter()
        .flat_map(|c| c.sysctls.iter().flatten());
    for sysctl in sysctls {
        if !SAFE_SYSCTLS.contains(&sysctl.name.as_str()) {
            let name = sysctl.name.clone();
            found.push(at, LintRule::UnsafeSysctl { name });
        }
    }
    let ephemeral = spec.ephemeral_containers.iter().flatten().map(|c| {
        (
            false,
            Container {
                name: c.name.clone(),
                image: c.image.clone(),
                resources: c.resources.clone(),
                volume_mounts: c.volume_mounts.clone(),
                ..Container::default()
            },
        )
    });
    let containers = spec
        .init_containers
        .iter()
        .flatten()
        .map(|c| (false, c.clone()))
        .chain(spec.containers.iter().map(|c| (true, c.clone())))
        .chain(ephemeral);
    for (regular, container) in containers {
        let at = &format!("{at}/{}", container.name);
        if !pinned(container.image.as_deref().unwrap_or_default()) {
            found.push(at, LintRule::ImageDigest);
        }
        let mounts = container.volume_mounts.iter().flatten();
        for mount in mounts {
            if mount.mount_propagation.as_deref() == Some("Bidirectional") {
                found.push(at, LintRule::BidirectionalMount);
            }
        }
        requirements(container.resources.as_ref(), at, found);
        if regular
            && probes
            && (container.readiness_probe.is_none() || container.liveness_probe.is_none())
        {
            found.push(at, LintRule::ProbesRequired);
        }
    }
}

/// Adds a [`LintRule::PositiveResource`] for each CPU and memory request and limit that is
/// missing or not above zero.
pub(super) fn requirements(resources: Option<&ResourceRequirements>, at: &str, found: &mut Found) {
    let levels = [
        ("requests", resources.and_then(|r| r.requests.as_ref())),
        ("limits", resources.and_then(|r| r.limits.as_ref())),
    ];
    for (level, list) in levels {
        for key in ["cpu", "memory"] {
            let positive = list
                .and_then(|l| l.get(key))
                .and_then(|q| Quantity::parse(&q.0))
                .is_some_and(|q| q.is_positive());
            if !positive {
                let resource = format!("{level}.{key}");
                found.push(at, LintRule::PositiveResource { resource });
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use zeabur_catalog_format::problem::LintRule;

    use super::super::tests::{SAFE, allow, assert_breaks, digest, replace, safety, template};

    #[test]
    fn privilege_and_network_escapes_break_rules() {
        let app = template(&[("app", "restricted")], &[]);
        let traefik = "{namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: traefik}}, \
                       podSelector: {matchLabels: {app: traefik}}}";
        assert_eq!(safety(&app, &(SAFE.to_owned() + &allow(traefik))), []);
        let client = "{podSelector: {matchLabels: {templates.zeabur.com/app-client: 'true'}}}";
        assert_eq!(safety(&app, &(SAFE.to_owned() + &allow(client))), []);

        let token = "automountServiceAccountToken: false";
        let escalation = "allowPrivilegeEscalation: false";
        let cases = [
            (
                replace(SAFE, token, "automountServiceAccountToken: true"),
                "token",
            ),
            (
                replace(
                    SAFE,
                    escalation,
                    &format!("{escalation}\n            privileged: true"),
                ),
                "privileged",
            ),
            (
                replace(
                    SAFE,
                    escalation,
                    &format!("{escalation}\n            runAsNonRoot: false"),
                ),
                "nonroot",
            ),
            (
                replace(SAFE, token, &format!("{token}\n      hostPID: true")),
                "hostnamespace",
            ),
            (
                replace(
                    SAFE,
                    token,
                    &format!("{token}\n      volumes: [{{name: host, hostPath: {{path: /}}}}]"),
                ),
                "hostpath",
            ),
            (
                replace(
                    SAFE,
                    "          readinessProbe:",
                    "          lifecycle: {preStop: {httpGet: {host: 127.0.0.1, path: /, port: \
                     10250}}}\n          readinessProbe:",
                ),
                "lifecycle-host",
            ),
        ];
        for (manifest, name) in &cases {
            let rules = safety(&app, manifest);
            let want = if *name == "token" {
                matches!(rules.as_slice(), [LintRule::PodTokenAutomount])
            } else {
                rules
                    .iter()
                    .any(|r| matches!(r, LintRule::PodSecurity { .. }))
            };
            assert!(want, "{name}: {rules:?}");
        }

        let tag = replace(
            SAFE,
            &format!("example.test/app{}", digest()),
            "example.test/app:latest",
        );
        assert_breaks!(safety(&app, &tag), LintRule::ImageDigest);
        let namespace = replace(
            SAFE,
            "name: app\nspec:",
            "name: app\n  namespace: kube-system\nspec:",
        );
        assert_breaks!(safety(&app, &namespace), LintRule::NamespaceEscape);
        let label = replace(
            SAFE,
            "labels: {app: app, templates.zeabur.com/template: app}",
            "labels: {app: app}",
        );
        assert_breaks!(safety(&app, &label), LintRule::TemplateLabel);
        for peer in [
            "{podSelector: {matchLabels: {app: other}}}",
            "{podSelector: {matchLabels: {templates.zeabur.com/other-client: 'true'}}}",
            "{namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: test}}}",
        ] {
            let rules = safety(&app, &(SAFE.to_owned() + &allow(peer)));
            assert_breaks!(rules, LintRule::ForeignPods { .. });
        }
        let claim = "---\napiVersion: v1\nkind: PersistentVolumeClaim\nmetadata: {name: data}\n\
                     spec: {storageClassName: local-path}\n";
        assert_breaks!(
            safety(&app, &(SAFE.to_owned() + claim)),
            LintRule::StorageClass
        );
        let binding = "---\napiVersion: rbac.authorization.k8s.io/v1\nkind: RoleBinding\n\
                       metadata: {name: escape}\n";
        assert_breaks!(
            safety(&app, &(SAFE.to_owned() + binding)),
            LintRule::UnsupportedKind
        );
    }

    #[test]
    fn a_baseline_service_does_not_lower_an_unrelated_workload() {
        let app = template(&[("app", "restricted"), ("legacy", "baseline")], &[]);
        let root = replace(SAFE, "runAsNonRoot: true", "runAsNonRoot: false");
        assert_breaks!(safety(&app, &root), LintRule::PodSecurity { .. });
    }

    #[test]
    fn restricted_allows_isolated_ping_but_no_host_sysctl() {
        let app = template(&[("app", "restricted")], &[]);
        let sysctl = |name: &str, value: &str| {
            replace(
                SAFE,
                "securityContext:\n        runAsNonRoot: true",
                &format!(
                    "securityContext:\n        sysctls: [{{name: {name}, value: \"{value}\"}}]\n        \
                     runAsNonRoot: true"
                ),
            )
        };
        assert_eq!(
            safety(&app, &sysctl("net.ipv4.ping_group_range", "1000 1000")),
            []
        );
        let rules = safety(&app, &sysctl("kernel.core_pattern", "/tmp/core"));
        assert_breaks!(rules, LintRule::UnsafeSysctl { .. });
    }

    #[test]
    fn an_unauthenticated_service_needs_an_ingress_policy() {
        let mut app = template(&[("app", "restricted"), ("cache", "restricted")], &[]);
        assert_eq!(safety(&app, SAFE), []);
        app.security.unauthenticated = vec!["app".to_owned()];
        assert_breaks!(safety(&app, SAFE), LintRule::IngressPolicy { .. });
        let named = replace(
            &allow("{podSelector: {matchLabels: {templates.zeabur.com/template: app}}}"),
            "template: app}}\n  policyTypes",
            "template: app, app: app}}\n  policyTypes",
        );
        let manifest = SAFE.to_owned() + &named;
        assert_eq!(safety(&app, &manifest), []);
        app.security.unauthenticated = vec!["cache".to_owned()];
        assert_breaks!(safety(&app, &manifest), LintRule::IngressPolicy { .. });
        app.security.unauthenticated = vec!["missing".to_owned()];
        assert_breaks!(safety(&app, SAFE), LintRule::UnknownService { .. });
    }

    #[test]
    fn a_core_object_that_does_not_parse_is_invalid() {
        let app = template(&[("app", "restricted")], &[]);
        let broken = "---\napiVersion: v1\nkind: Service\nmetadata: {name: web}\n\
                      spec: {ports: not-a-list}\n";
        let rules = safety(&app, &(SAFE.to_owned() + broken));
        assert_breaks!(rules, LintRule::Invalid { kind, .. } if kind == "Service");
    }

    /// A `Job` or `CronJob` without probes, with `privileged`, `image` and resources or not.
    fn finite(kind: &str, privileged: bool, image: &str, resources: bool) -> String {
        let privileged = if privileged { ", privileged: true" } else { "" };
        let resources = if resources {
            "\n        resources: {requests: {cpu: 10m, memory: 16Mi}, limits: {cpu: 100m, memory: 32Mi}}"
        } else {
            ""
        };
        let job = format!(
            "template:
  metadata:
    labels: {{app: app, templates.zeabur.com/template: app}}
  spec:
    restartPolicy: Never
    automountServiceAccountToken: false
    securityContext: {{runAsNonRoot: true, seccompProfile: {{type: RuntimeDefault}}}}
    containers:
      - name: app
        image: {image}
        securityContext: {{allowPrivilegeEscalation: false, capabilities: {{drop: [ALL]}}{privileged}}}{resources}
"
        );
        let indent = |text: &str, by: &str| {
            text.lines()
                .fold(String::new(), |text, l| text + by + l + "\n")
        };
        let spec = if kind == "CronJob" {
            format!(
                "  schedule: 0 0 * * *\n  jobTemplate:\n    spec:\n{}",
                indent(&job, "      ")
            )
        } else {
            indent(&job, "  ")
        };
        format!("apiVersion: batch/v1\nkind: {kind}\nmetadata: {{name: app}}\nspec:\n{spec}")
    }

    /// A `Job` or `CronJob` runs to completion, so it needs no probes, but the rest of the pod
    /// rules hold for it.
    #[test]
    fn finite_jobs_need_no_probes() {
        let app = template(&[("app", "restricted")], &[]);
        let probeless = replace(
            &replace(
                SAFE,
                "          readinessProbe: {tcpSocket: {port: 8080}}\n",
                "",
            ),
            "          livenessProbe: {tcpSocket: {port: 8080}}\n",
            "",
        );
        assert_breaks!(safety(&app, &probeless), LintRule::ProbesRequired);

        let pinned = format!("example.test/app{}", digest());
        for kind in ["Job", "CronJob"] {
            assert_eq!(
                safety(&app, &finite(kind, false, &pinned, true)),
                [],
                "{kind}"
            );
            let rules = safety(&app, &finite(kind, true, &pinned, true));
            assert_breaks!(rules, LintRule::PodSecurity { .. });
            let rules = safety(&app, &finite(kind, false, "example.test/app:latest", true));
            assert_breaks!(rules, LintRule::ImageDigest);
            let rules = safety(&app, &finite(kind, false, &pinned, false));
            assert_breaks!(rules, LintRule::PositiveResource { .. });
        }
    }
}
