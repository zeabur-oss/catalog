//! The [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
//! checks of Pod Security Admission, at their latest version.
//!
//! Each check, its order and its message follow `k8s.io/pod-security-admission/policy` v0.37.2,
//! so a finding reads as the API server's own refusal.

use std::collections::BTreeSet;
use std::fmt;

use k8s_openapi::api::apps::v1::{DaemonSet, Deployment, ReplicaSet, StatefulSet};
use k8s_openapi::api::batch::v1::{CronJob, Job};
use k8s_openapi::api::core::v1::{
    Container, LifecycleHandler, Pod, PodSpec, PodTemplateSpec, Probe, SecurityContext, Volume,
};
use k8s_openapi::apimachinery::pkg::apis::meta::v1::ObjectMeta;
use serde::Deserialize;
use serde::de::DeserializeOwned;
use zeabur_catalog_format::common::PodSecurity;

/// One failed check: the reason and what to change.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Forbidden {
    /// Why the pod is refused, such as `privileged`.
    pub reason: &'static str,
    /// Which fields to change.
    pub detail: String,
}

/// The failed checks of one pod, written as Pod Security Admission writes them.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Refusal(pub Vec<Forbidden>);

impl fmt::Display for Refusal {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for (i, Forbidden { reason, detail }) in self.0.iter().enumerate() {
            if i > 0 {
                f.write_str(", ")?;
            }
            f.write_str(reason)?;
            if !detail.is_empty() {
                write!(f, " ({detail})")?;
            }
        }
        Ok(())
    }
}

type Check = fn(&ObjectMeta, &PodSpec) -> Option<Forbidden>;

/// Baseline checks sorted by their upstream ID, then the restricted ones. A restricted check
/// replaces the baseline check named next to it.
const BASELINE: [(&str, Check); 12] = [
    ("appArmorProfile", app_armor_profile),
    ("capabilities_baseline", capabilities_baseline),
    ("hostNamespaces", host_namespaces),
    ("hostPathVolumes", host_path_volumes),
    ("hostPorts", host_ports),
    ("hostProbesAndHostLifecycle", host_probes_and_lifecycle),
    ("privileged", privileged),
    ("procMount", proc_mount_baseline),
    ("seLinuxOptions", se_linux_options),
    ("seccompProfile_baseline", seccomp_baseline),
    ("sysctls", sysctls),
    ("windowsHostProcess", windows_host_process),
];
const RESTRICTED: [(Option<&str>, Check); 7] = [
    (None, allow_privilege_escalation),
    (Some("capabilities_baseline"), capabilities_restricted),
    (Some("procMount"), proc_mount),
    (Some("hostPathVolumes"), restricted_volumes),
    (None, run_as_non_root),
    (None, run_as_user),
    (Some("seccompProfile_baseline"), seccomp_restricted),
];

/// Checks a pod against `level`. A level this reader does not know is checked as restricted.
#[must_use]
pub fn check(level: PodSecurity, meta: &ObjectMeta, spec: &PodSpec) -> Option<Refusal> {
    let restricted = match level {
        PodSecurity::Privileged => return None,
        PodSecurity::Baseline => false,
        PodSecurity::Restricted | PodSecurity::Unknown => true,
    };
    let replaced: Vec<&str> = if restricted {
        RESTRICTED.iter().filter_map(|(id, _)| *id).collect()
    } else {
        vec![]
    };
    let checks = BASELINE
        .iter()
        .filter(|(id, _)| !replaced.contains(id))
        .map(|(_, c)| c)
        .chain(RESTRICTED.iter().filter(|_| restricted).map(|(_, c)| c));
    let forbidden: Vec<Forbidden> = checks.filter_map(|c| c(meta, spec)).collect();
    (!forbidden.is_empty()).then_some(Refusal(forbidden))
}

/// The pod template of a workload, or `None` for a kind that runs no pods.
///
/// # Errors
///
/// The object does not read as its kind.
#[expect(
    clippy::disallowed_types,
    reason = "a rendered object may be of any kind; its kind picks the type it reads as"
)]
pub fn pod_template(
    object: &serde_json::Value,
) -> Result<Option<PodTemplateSpec>, serde_json::Error> {
    #[derive(Deserialize)]
    struct Kind {
        #[serde(default)]
        kind: String,
    }
    fn read<T: DeserializeOwned>(object: &serde_json::Value) -> Result<T, serde_json::Error> {
        T::deserialize(object)
    }
    let template = match Kind::deserialize(object)?.kind.as_str() {
        "Pod" => {
            let pod: Pod = read(object)?;
            Some(PodTemplateSpec {
                metadata: Some(pod.metadata),
                spec: pod.spec,
            })
        }
        "Deployment" => read::<Deployment>(object)?.spec.map(|s| s.template),
        "StatefulSet" => read::<StatefulSet>(object)?.spec.map(|s| s.template),
        "DaemonSet" => read::<DaemonSet>(object)?.spec.map(|s| s.template),
        "ReplicaSet" => read::<ReplicaSet>(object)?.spec.and_then(|s| s.template),
        "Job" => read::<Job>(object)?.spec.map(|s| s.template),
        "CronJob" => read::<CronJob>(object)?
            .spec
            .and_then(|s| s.job_template.spec)
            .map(|s| s.template),
        _ => None,
    };
    Ok(template)
}

/// Init, regular and ephemeral containers, in that order.
fn containers(spec: &PodSpec) -> Vec<Container> {
    let ephemeral = spec
        .ephemeral_containers
        .iter()
        .flatten()
        .map(|e| Container {
            name: e.name.clone(),
            ports: e.ports.clone(),
            security_context: e.security_context.clone(),
            liveness_probe: e.liveness_probe.clone(),
            readiness_probe: e.readiness_probe.clone(),
            startup_probe: e.startup_probe.clone(),
            lifecycle: e.lifecycle.clone(),
            ..Container::default()
        });
    spec.init_containers
        .iter()
        .flatten()
        .chain(&spec.containers)
        .cloned()
        .chain(ephemeral)
        .collect()
}

/// The names of the containers whose security context matches `bad`.
fn bad_containers(spec: &PodSpec, bad: impl Fn(Option<&SecurityContext>) -> bool) -> Vec<String> {
    containers(spec)
        .into_iter()
        .filter(|c| bad(c.security_context.as_ref()))
        .map(|c| c.name)
        .collect()
}

fn join_quote<S: AsRef<str>>(items: impl IntoIterator<Item = S>) -> String {
    items
        .into_iter()
        .map(|s| format!("\"{}\"", s.as_ref()))
        .collect::<Vec<_>>()
        .join(", ")
}

const fn plural(singular: &'static str, plural: &'static str, n: usize) -> &'static str {
    if n == 1 { singular } else { plural }
}

/// `containers "a", "b"`.
fn named(names: &[String]) -> String {
    format!(
        "{} {}",
        plural("container", "containers", names.len()),
        join_quote(names)
    )
}

fn windows(spec: &PodSpec) -> bool {
    spec.os.as_ref().is_some_and(|os| os.name == "windows")
}

/// A pod in a user namespace (`hostUsers: false`) may run as root and use any `procMount`.
fn user_namespace(spec: &PodSpec) -> bool {
    spec.host_users == Some(false)
}

const fn forbidden(reason: &'static str, detail: String) -> Forbidden {
    Forbidden { reason, detail }
}

fn allow_privilege_escalation(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    if windows(spec) {
        return None;
    }
    let bad = bad_containers(spec, |s| {
        s.and_then(|s| s.allow_privilege_escalation) != Some(false)
    });
    if bad.is_empty() {
        return None;
    }
    Some(forbidden(
        "allowPrivilegeEscalation != false",
        format!(
            "{} must set securityContext.allowPrivilegeEscalation=false",
            named(&bad)
        ),
    ))
}

fn app_armor_profile(meta: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let allowed = |t: &str| matches!(t, "RuntimeDefault" | "Localhost");
    let mut setters = vec![];
    let mut values = BTreeSet::new();
    if let Some(p) = spec
        .security_context
        .as_ref()
        .and_then(|s| s.app_armor_profile.as_ref())
        && !allowed(&p.type_)
    {
        setters.push("pod".to_owned());
        values.insert(p.type_.clone());
    }
    let mut bad = vec![];
    for c in containers(spec) {
        if let Some(p) = c.security_context.and_then(|s| s.app_armor_profile)
            && !allowed(&p.type_)
        {
            bad.push(c.name);
            values.insert(p.type_);
        }
    }
    if !bad.is_empty() {
        setters.push(named(&bad));
    }
    let mut values: Vec<String> = values.into_iter().collect();
    let mut annotations: Vec<String> = meta
        .annotations
        .iter()
        .flatten()
        .filter(|(k, v)| {
            k.starts_with("container.apparmor.security.beta.kubernetes.io/")
                && !(v.is_empty() || *v == "runtime/default" || v.starts_with("localhost/"))
        })
        .map(|(k, v)| format!("{k}={v:?}"))
        .collect();
    if !annotations.is_empty() {
        annotations.sort();
        setters.push(plural("annotation", "annotations", annotations.len()).to_owned());
        values.append(&mut annotations);
    }
    if setters.is_empty() {
        return None;
    }
    Some(forbidden(
        plural(
            "forbidden AppArmor profile",
            "forbidden AppArmor profiles",
            values.len(),
        ),
        format!(
            "{} must not set AppArmor profile type to {}",
            setters.join(" and "),
            join_quote(&values)
        ),
    ))
}

const CAPABILITIES_BASELINE: [&str; 13] = [
    "AUDIT_WRITE",
    "CHOWN",
    "DAC_OVERRIDE",
    "FOWNER",
    "FSETID",
    "KILL",
    "MKNOD",
    "NET_BIND_SERVICE",
    "SETFCAP",
    "SETGID",
    "SETPCAP",
    "SETUID",
    "SYS_CHROOT",
];

fn added(c: &Container) -> &[String] {
    c.security_context
        .as_ref()
        .and_then(|s| s.capabilities.as_ref())
        .and_then(|c| c.add.as_deref())
        .unwrap_or_default()
}

/// Containers that add a capability outside `allowed`, and those capabilities.
fn adding(spec: &PodSpec, allowed: &[&str]) -> (Vec<String>, BTreeSet<String>) {
    let mut bad = vec![];
    let mut capabilities = BTreeSet::new();
    for c in containers(spec) {
        let extra: Vec<&String> = added(&c)
            .iter()
            .filter(|a| !allowed.contains(&a.as_str()))
            .collect();
        if !extra.is_empty() {
            capabilities.extend(extra.into_iter().cloned());
            bad.push(c.name);
        }
    }
    (bad, capabilities)
}

fn capabilities_baseline(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let (bad, capabilities) = adding(spec, &CAPABILITIES_BASELINE);
    if bad.is_empty() {
        return None;
    }
    Some(forbidden(
        "non-default capabilities",
        format!(
            "{} must not include {} in securityContext.capabilities.add",
            named(&bad),
            join_quote(&capabilities)
        ),
    ))
}

fn capabilities_restricted(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    if windows(spec) {
        return None;
    }
    let keep_all: Vec<String> = containers(spec)
        .into_iter()
        .filter(|c| {
            let drop = c
                .security_context
                .as_ref()
                .and_then(|s| s.capabilities.as_ref());
            !drop
                .and_then(|c| c.drop.as_ref())
                .is_some_and(|d| d.iter().any(|d| d == "ALL"))
        })
        .map(|c| c.name)
        .collect();
    let (adding, capabilities) = adding(spec, &["NET_BIND_SERVICE"]);
    let mut details = vec![];
    if !keep_all.is_empty() {
        details.push(format!(
            "{} must set securityContext.capabilities.drop=[\"ALL\"]",
            named(&keep_all)
        ));
    }
    if !adding.is_empty() {
        details.push(format!(
            "{} must not include {} in securityContext.capabilities.add",
            named(&adding),
            join_quote(&capabilities)
        ));
    }
    if details.is_empty() {
        return None;
    }
    Some(forbidden("unrestricted capabilities", details.join("; ")))
}

fn host_namespaces(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let set: Vec<&str> = [
        (spec.host_network, "hostNetwork=true"),
        (spec.host_pid, "hostPID=true"),
        (spec.host_ipc, "hostIPC=true"),
    ]
    .into_iter()
    .filter(|(on, _)| *on == Some(true))
    .map(|(_, s)| s)
    .collect();
    if set.is_empty() {
        return None;
    }
    Some(forbidden("host namespaces", set.join(", ")))
}

fn host_path_volumes(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let bad: Vec<&str> = spec
        .volumes
        .iter()
        .flatten()
        .filter(|v| v.host_path.is_some())
        .map(|v| v.name.as_str())
        .collect();
    if bad.is_empty() {
        return None;
    }
    Some(forbidden(
        "hostPath volumes",
        format!(
            "{} {}",
            plural("volume", "volumes", bad.len()),
            join_quote(&bad)
        ),
    ))
}

fn host_ports(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let mut bad = vec![];
    let mut ports = BTreeSet::new();
    for c in containers(spec) {
        let used: Vec<String> = c
            .ports
            .iter()
            .flatten()
            .filter_map(|p| p.host_port.filter(|p| *p != 0))
            .map(|p| p.to_string())
            .collect();
        if !used.is_empty() {
            ports.extend(used);
            bad.push(c.name);
        }
    }
    if bad.is_empty() {
        return None;
    }
    Some(forbidden(
        "hostPort",
        format!(
            "{} {} {} {}",
            named(&bad),
            plural("uses", "use", bad.len()),
            plural("hostPort", "hostPorts", ports.len()),
            ports.into_iter().collect::<Vec<_>>().join(", ")
        ),
    ))
}

fn probe_hosts(p: Option<&Probe>) -> impl Iterator<Item = &String> {
    p.into_iter().flat_map(|p| {
        let http = p.http_get.as_ref().and_then(|h| h.host.as_ref());
        let tcp = p.tcp_socket.as_ref().and_then(|t| t.host.as_ref());
        http.into_iter().chain(tcp).filter(|h| !h.is_empty())
    })
}

fn handler_hosts(h: Option<&LifecycleHandler>) -> impl Iterator<Item = &String> {
    h.into_iter().flat_map(|h| {
        let http = h.http_get.as_ref().and_then(|h| h.host.as_ref());
        let tcp = h.tcp_socket.as_ref().and_then(|t| t.host.as_ref());
        http.into_iter().chain(tcp).filter(|h| !h.is_empty())
    })
}

fn host_probes_and_lifecycle(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let mut bad = BTreeSet::new();
    let mut hosts = BTreeSet::new();
    for c in containers(spec) {
        let lifecycle = c.lifecycle.as_ref();
        let found: Vec<&String> = probe_hosts(c.liveness_probe.as_ref())
            .chain(probe_hosts(c.readiness_probe.as_ref()))
            .chain(probe_hosts(c.startup_probe.as_ref()))
            .chain(handler_hosts(lifecycle.and_then(|l| l.post_start.as_ref())))
            .chain(handler_hosts(lifecycle.and_then(|l| l.pre_stop.as_ref())))
            .collect();
        if !found.is_empty() {
            hosts.extend(found.into_iter().cloned());
            bad.insert(c.name.clone());
        }
    }
    if bad.is_empty() {
        return None;
    }
    let bad: Vec<String> = bad.into_iter().collect();
    Some(forbidden(
        "probe or lifecycle host",
        format!(
            "{} {} {} {}",
            named(&bad),
            plural("uses", "use", bad.len()),
            plural(
                "probe or lifecycle host",
                "probe or lifecycle hosts",
                hosts.len()
            ),
            join_quote(&hosts)
        ),
    ))
}

fn privileged(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let bad = bad_containers(spec, |s| s.and_then(|s| s.privileged) == Some(true));
    if bad.is_empty() {
        return None;
    }
    Some(forbidden(
        "privileged",
        format!(
            "{} must not set securityContext.privileged=true",
            named(&bad)
        ),
    ))
}

fn proc_mount(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let mut bad = vec![];
    let mut types = BTreeSet::new();
    for c in containers(spec) {
        if let Some(t) = c.security_context.and_then(|s| s.proc_mount)
            && t != "Default"
        {
            bad.push(c.name);
            types.insert(t);
        }
    }
    if bad.is_empty() {
        return None;
    }
    Some(forbidden(
        "procMount",
        format!(
            "{} must not set securityContext.procMount to {}",
            named(&bad),
            join_quote(&types)
        ),
    ))
}

fn proc_mount_baseline(meta: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    if user_namespace(spec) {
        return None;
    }
    proc_mount(meta, spec)
}

/// The name of the source of a volume that the restricted level forbids.
fn restricted_source(v: &Volume) -> Option<&'static str> {
    let allowed = v.config_map.is_some()
        || v.csi.is_some()
        || v.downward_api.is_some()
        || v.empty_dir.is_some()
        || v.ephemeral.is_some()
        || v.image.is_some()
        || v.persistent_volume_claim.is_some()
        || v.projected.is_some()
        || v.secret.is_some();
    if allowed {
        return None;
    }
    let sources = [
        (v.host_path.is_some(), "hostPath"),
        (v.gce_persistent_disk.is_some(), "gcePersistentDisk"),
        (v.aws_elastic_block_store.is_some(), "awsElasticBlockStore"),
        (v.git_repo.is_some(), "gitRepo"),
        (v.nfs.is_some(), "nfs"),
        (v.iscsi.is_some(), "iscsi"),
        (v.glusterfs.is_some(), "glusterfs"),
        (v.rbd.is_some(), "rbd"),
        (v.flex_volume.is_some(), "flexVolume"),
        (v.cinder.is_some(), "cinder"),
        (v.cephfs.is_some(), "cephfs"),
        (v.flocker.is_some(), "flocker"),
        (v.fc.is_some(), "fc"),
        (v.azure_file.is_some(), "azureFile"),
        (v.vsphere_volume.is_some(), "vsphereVolume"),
        (v.quobyte.is_some(), "quobyte"),
        (v.azure_disk.is_some(), "azureDisk"),
        (v.photon_persistent_disk.is_some(), "photonPersistentDisk"),
        (v.portworx_volume.is_some(), "portworxVolume"),
        (v.scale_io.is_some(), "scaleIO"),
        (v.storageos.is_some(), "storageos"),
    ];
    Some(
        sources
            .into_iter()
            .find(|(set, _)| *set)
            .map_or("unknown", |(_, name)| name),
    )
}

fn restricted_volumes(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let mut bad = vec![];
    let mut types = BTreeSet::new();
    for v in spec.volumes.iter().flatten() {
        if let Some(source) = restricted_source(v) {
            bad.push(v.name.as_str());
            types.insert(source);
        }
    }
    if bad.is_empty() {
        return None;
    }
    Some(forbidden(
        "restricted volume types",
        format!(
            "{} {} {} {} {}",
            plural("volume", "volumes", bad.len()),
            join_quote(&bad),
            plural("uses", "use", bad.len()),
            plural(
                "restricted volume type",
                "restricted volume types",
                types.len()
            ),
            join_quote(&types)
        ),
    ))
}

fn run_as_non_root(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    if user_namespace(spec) {
        return None;
    }
    let pod = spec
        .security_context
        .as_ref()
        .and_then(|s| s.run_as_non_root);
    let mut setters = vec![];
    if pod == Some(false) {
        setters.push("pod".to_owned());
    }
    let mut explicit = vec![];
    let mut implicit = vec![];
    for c in containers(spec) {
        match c.security_context.and_then(|s| s.run_as_non_root) {
            Some(false) => explicit.push(c.name),
            None if pod != Some(true) => implicit.push(c.name),
            Some(true) | None => {}
        }
    }
    if !explicit.is_empty() {
        setters.push(named(&explicit));
    }
    if !setters.is_empty() {
        return Some(forbidden(
            "runAsNonRoot != true",
            format!(
                "{} must not set securityContext.runAsNonRoot=false",
                setters.join(" and ")
            ),
        ));
    }
    if implicit.is_empty() {
        return None;
    }
    Some(forbidden(
        "runAsNonRoot != true",
        format!(
            "pod or {} must set securityContext.runAsNonRoot=true",
            named(&implicit)
        ),
    ))
}

fn run_as_user(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    if user_namespace(spec) {
        return None;
    }
    let mut setters = vec![];
    if spec.security_context.as_ref().and_then(|s| s.run_as_user) == Some(0) {
        setters.push("pod".to_owned());
    }
    let bad = bad_containers(spec, |s| s.and_then(|s| s.run_as_user) == Some(0));
    if !bad.is_empty() {
        setters.push(named(&bad));
    }
    if setters.is_empty() {
        return None;
    }
    Some(forbidden(
        "runAsUser=0",
        format!("{} must not set runAsUser=0", setters.join(" and ")),
    ))
}

fn se_linux_options(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    const TYPES: [&str; 5] = [
        "",
        "container_t",
        "container_init_t",
        "container_kvm_t",
        "container_engine_t",
    ];
    let mut types = BTreeSet::new();
    let (mut user, mut role) = (false, false);
    let mut valid = |o: &k8s_openapi::api::core::v1::SELinuxOptions| {
        let t = o.type_.clone().unwrap_or_default();
        let good_type = TYPES.contains(&t.as_str());
        if !good_type {
            types.insert(t);
        }
        let set_user = o.user.as_ref().is_some_and(|u| !u.is_empty());
        let set_role = o.role.as_ref().is_some_and(|r| !r.is_empty());
        user |= set_user;
        role |= set_role;
        good_type && !set_user && !set_role
    };
    let mut setters = vec![];
    if let Some(o) = spec
        .security_context
        .as_ref()
        .and_then(|s| s.se_linux_options.as_ref())
        && !valid(o)
    {
        setters.push("pod".to_owned());
    }
    let mut bad = vec![];
    for c in containers(spec) {
        if let Some(o) = c
            .security_context
            .as_ref()
            .and_then(|s| s.se_linux_options.as_ref())
            && !valid(o)
        {
            bad.push(c.name);
        }
    }
    if !bad.is_empty() {
        setters.push(named(&bad));
    }
    if setters.is_empty() {
        return None;
    }
    let mut data = vec![];
    if !types.is_empty() {
        data.push(format!(
            "{} {}",
            plural("type", "types", types.len()),
            join_quote(&types)
        ));
    }
    if user {
        data.push("user may not be set".to_owned());
    }
    if role {
        data.push("role may not be set".to_owned());
    }
    Some(forbidden(
        "seLinuxOptions",
        format!(
            "{} set forbidden securityContext.seLinuxOptions: {}",
            setters.join(" and "),
            data.join("; ")
        ),
    ))
}

fn seccomp_type(s: Option<&SecurityContext>) -> Option<&str> {
    s.and_then(|s| s.seccomp_profile.as_ref())
        .map(|p| p.type_.as_str())
}

fn valid_seccomp(t: &str) -> bool {
    matches!(t, "Localhost" | "RuntimeDefault")
}

/// Seccomp checks of both levels. `required` also refuses containers that set no profile when
/// the pod sets none.
fn seccomp(spec: &PodSpec, required: bool) -> Option<Forbidden> {
    let pod = spec
        .security_context
        .as_ref()
        .and_then(|s| s.seccomp_profile.as_ref());
    let mut setters = vec![];
    let mut values = BTreeSet::new();
    if let Some(p) = pod
        && !valid_seccomp(&p.type_)
    {
        setters.push("pod".to_owned());
        values.insert(p.type_.clone());
    }
    let pod_set = pod.is_some_and(|p| valid_seccomp(&p.type_));
    let mut explicit = vec![];
    let mut implicit = vec![];
    for c in containers(spec) {
        match seccomp_type(c.security_context.as_ref()) {
            Some(t) if !valid_seccomp(t) => {
                values.insert(t.to_owned());
                explicit.push(c.name);
            }
            None if !pod_set => implicit.push(c.name),
            Some(_) | None => {}
        }
    }
    if !explicit.is_empty() {
        setters.push(named(&explicit));
    }
    if !setters.is_empty() {
        return Some(forbidden(
            "seccompProfile",
            format!(
                "{} must not set securityContext.seccompProfile.type to {}",
                setters.join(" and "),
                join_quote(&values)
            ),
        ));
    }
    if !required || implicit.is_empty() {
        return None;
    }
    Some(forbidden(
        "seccompProfile",
        format!(
            "pod or {} must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\"",
            named(&implicit)
        ),
    ))
}

fn seccomp_baseline(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    seccomp(spec, false)
}

fn seccomp_restricted(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    if windows(spec) {
        return None;
    }
    seccomp(spec, true)
}

const SYSCTLS: [&str; 14] = [
    "kernel.shm_rmid_forced",
    "net.ipv4.ip_local_port_range",
    "net.ipv4.tcp_syncookies",
    "net.ipv4.ping_group_range",
    "net.ipv4.ip_unprivileged_port_start",
    "net.ipv4.ip_local_reserved_ports",
    "net.ipv4.tcp_keepalive_time",
    "net.ipv4.tcp_fin_timeout",
    "net.ipv4.tcp_keepalive_intvl",
    "net.ipv4.tcp_keepalive_probes",
    "net.ipv4.tcp_rmem",
    "net.ipv4.tcp_wmem",
    "net.ipv4.tcp_slow_start_after_idle",
    "net.ipv4.tcp_notsent_lowat",
];

fn sysctls(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let bad: Vec<&str> = spec
        .security_context
        .iter()
        .flat_map(|s| s.sysctls.iter().flatten())
        .map(|s| s.name.as_str())
        .filter(|n| !SYSCTLS.contains(n))
        .collect();
    if bad.is_empty() {
        return None;
    }
    Some(forbidden("forbidden sysctls", bad.join(", ")))
}

fn windows_host_process(_: &ObjectMeta, spec: &PodSpec) -> Option<Forbidden> {
    let host_process = |s: Option<&SecurityContext>| {
        s.and_then(|s| s.windows_options.as_ref())
            .and_then(|w| w.host_process)
    };
    let mut setters = vec![];
    let pod = spec
        .security_context
        .as_ref()
        .and_then(|s| s.windows_options.as_ref());
    if pod.and_then(|w| w.host_process) == Some(true) {
        setters.push("pod".to_owned());
    }
    let bad = bad_containers(spec, |s| host_process(s) == Some(true));
    if !bad.is_empty() {
        setters.push(named(&bad));
    }
    if setters.is_empty() {
        return None;
    }
    Some(forbidden(
        "hostProcess",
        format!(
            "{} must not set securityContext.windowsOptions.hostProcess=true",
            setters.join(" and ")
        ),
    ))
}

#[cfg(test)]
mod tests {
    #![expect(
        clippy::disallowed_types,
        reason = "the cases are pods as JSON, the shape the renderer gives"
    )]

    use serde_json::{Value, json};

    use super::*;

    fn template(pod: &Value) -> PodTemplateSpec {
        pod_template(pod).unwrap().unwrap()
    }

    fn refusal(level: PodSecurity, template: PodTemplateSpec) -> Option<String> {
        let spec = template.spec.unwrap_or_default();
        check(level, &template.metadata.unwrap_or_default(), &spec).map(|r| r.to_string())
    }

    fn pod(spec: &Value) -> Value {
        json!({"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "p"}, "spec": spec})
    }

    /// A container that passes the restricted level.
    fn safe(name: &str) -> Value {
        json!({"name": name, "image": "i", "securityContext": {
            "allowPrivilegeEscalation": false, "runAsNonRoot": true,
            "capabilities": {"drop": ["ALL"]}, "seccompProfile": {"type": "RuntimeDefault"}}})
    }

    /// The expected messages are those of the upstream tests of `k8s.io/pod-security-admission`.
    #[test]
    fn refuses_a_pod_as_pod_security_admission_does() {
        let cases = [
            (
                PodSecurity::Restricted,
                json!({"containers": [{"name": "a", "image": "i"}]}),
                Some(
                    "allowPrivilegeEscalation != false (container \"a\" must set \
                     securityContext.allowPrivilegeEscalation=false), unrestricted capabilities \
                     (container \"a\" must set securityContext.capabilities.drop=[\"ALL\"]), \
                     runAsNonRoot != true (pod or container \"a\" must set \
                     securityContext.runAsNonRoot=true), seccompProfile (pod or container \"a\" \
                     must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \
                     \"Localhost\")",
                ),
            ),
            (
                PodSecurity::Restricted,
                json!({"containers": [safe("a")]}),
                None,
            ),
            (
                PodSecurity::Baseline,
                json!({"containers": [{"name": "a", "image": "i"}]}),
                None,
            ),
            (
                PodSecurity::Privileged,
                json!({"hostNetwork": true, "containers": [safe("a")]}),
                None,
            ),
            (
                PodSecurity::Baseline,
                json!({"containers": [
                    {"name": "a", "image": "i", "securityContext": {"capabilities": {"add": ["FOO", "BAR"]}}},
                    {"name": "b", "image": "i", "securityContext": {"capabilities": {"add": ["BAR", "BAZ"]}}}]}),
                Some(
                    "non-default capabilities (containers \"a\", \"b\" must not include \"BAR\", \
                     \"BAZ\", \"FOO\" in securityContext.capabilities.add)",
                ),
            ),
            (
                PodSecurity::Restricted,
                json!({"containers": [
                    {"name": "c", "image": "i", "securityContext": {"allowPrivilegeEscalation": false,
                     "runAsNonRoot": true, "seccompProfile": {"type": "RuntimeDefault"},
                     "capabilities": {"add": ["NET_BIND_SERVICE", "CHOWN"], "drop": ["ALL", "FOO"]}}}]}),
                Some(
                    "unrestricted capabilities (container \"c\" must not include \"CHOWN\" in \
                     securityContext.capabilities.add)",
                ),
            ),
            (
                PodSecurity::Baseline,
                json!({"hostNetwork": true, "hostIPC": true, "volumes": [{"name": "v", "hostPath": {"path": "/"}}],
                       "securityContext": {"sysctls": [{"name": "kernel.msgmax", "value": "1"}],
                                           "seLinuxOptions": {"type": "spc_t", "user": "u"}},
                       "containers": [{"name": "a", "image": "i",
                           "ports": [{"containerPort": 80, "hostPort": 8080}],
                           "livenessProbe": {"httpGet": {"port": 80, "host": "10.0.0.1"}},
                           "securityContext": {"privileged": true, "procMount": "Unmasked",
                               "appArmorProfile": {"type": "Unconfined"},
                               "seccompProfile": {"type": "Unconfined"}}}]}),
                Some(
                    "forbidden AppArmor profile (container \"a\" must not set AppArmor profile \
                     type to \"Unconfined\"), host namespaces (hostNetwork=true, hostIPC=true), \
                     hostPath volumes (volume \"v\"), hostPort (container \"a\" uses hostPort \
                     8080), probe or lifecycle host (container \"a\" uses probe or lifecycle \
                     host \"10.0.0.1\"), privileged (container \"a\" must not set \
                     securityContext.privileged=true), procMount (container \"a\" must not set \
                     securityContext.procMount to \"Unmasked\"), seLinuxOptions (pod set \
                     forbidden securityContext.seLinuxOptions: type \"spc_t\"; user may not be \
                     set), seccompProfile (container \"a\" must not set \
                     securityContext.seccompProfile.type to \"Unconfined\"), forbidden sysctls \
                     (kernel.msgmax)",
                ),
            ),
            (
                PodSecurity::Restricted,
                json!({"hostUsers": false, "securityContext": {"runAsUser": 0},
                       "volumes": [{"name": "n", "nfs": {"server": "s", "path": "/"}}],
                       "containers": [safe("a")]}),
                Some("restricted volume types (volume \"n\" uses restricted volume type \"nfs\")"),
            ),
        ];
        for (level, spec, want) in cases {
            assert_eq!(
                refusal(level, template(&pod(&spec))).as_deref(),
                want,
                "{spec}"
            );
        }
    }

    #[test]
    fn finds_the_pod_of_each_workload() {
        let spec = json!({"containers": [{"name": "a", "image": "i"}]});
        let template = json!({"metadata": {"labels": {"a": "b"}}, "spec": spec});
        for workload in [
            json!({"kind": "Deployment", "spec": {"selector": {}, "template": template}}),
            json!({"kind": "CronJob", "spec": {"schedule": "* * * * *",
                   "jobTemplate": {"spec": {"template": template}}}}),
        ] {
            let found = pod_template(&workload).unwrap().unwrap();
            assert_eq!(found.spec.unwrap().containers[0].name, "a");
        }
        assert!(
            pod_template(&json!({"kind": "ConfigMap"}))
                .unwrap()
                .is_none()
        );
    }
}
