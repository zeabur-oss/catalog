//! The rules for the custom resources of the operators a template may use. A template renders
//! them only when it requires the component that provides the operator, and the rules here
//! check what the operator copies into the pods it creates.

use std::collections::BTreeMap;
use std::net::IpAddr;

use k8s_openapi::api::core::v1::{
    Container, PodSecurityContext, PodSpec, ResourceRequirements, SecurityContext, ServiceAccount,
};
use k8s_openapi::apimachinery::pkg::apis::meta::v1::ObjectMeta;
use serde::Deserialize;
use serde::de::IgnoredAny;
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
use serde_json::Value;
use zeabur_catalog_engine::template::TEMPLATE_LABEL;
use zeabur_catalog_format::common::PodSecurity;
use zeabur_catalog_format::problem::LintRule;

use super::safety::{pod_security, requirements};
use super::view::{Head, Keys, Labelled, Labels, Loose, Obj, Set, Spec, Zero, view};
use super::{Found, Subject};
use crate::render::pinned;

/// One rendered custom resource, and what it is checked with.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
struct Object<'a> {
    subject: &'a Subject<'a>,
    doc: &'a Value,
    head: &'a Head,
    docs: &'a [Value],
    at: &'a str,
}

/// What the lint knows about the kinds an operator owns: the component the template must
/// require, the rule it breaks when it does not, and the rest of the rules.
struct KindRule {
    api_version: &'static str,
    kinds: &'static [&'static str],
    provider: &'static str,
    missing: LintRule,
    check: Option<fn(&Object<'_>, &mut Found)>,
}

/// One entry for each operator kind. A CloudNativePG Cluster has two.
const RULES: [KindRule; 9] = [
    KindRule {
        api_version: "postgresql.cnpg.io/v1",
        kinds: &["Cluster"],
        provider: "cloudnative-pg",
        missing: LintRule::CnpgProvider,
        check: Some(cnpg_cluster),
    },
    KindRule {
        api_version: "postgresql.cnpg.io/v1",
        kinds: &["Cluster", "Backup", "ScheduledBackup"],
        provider: "cloudnative-pg",
        missing: LintRule::CnpgResourceProvider,
        check: None,
    },
    KindRule {
        api_version: "barmancloud.cnpg.io/v1",
        kinds: &["ObjectStore"],
        provider: "cloudnative-pg",
        missing: LintRule::CnpgResourceProvider,
        check: Some(barman_object_store),
    },
    KindRule {
        api_version: "psmdb.percona.com/v1",
        kinds: &["PerconaServerMongoDB"],
        provider: "percona-mongodb",
        missing: LintRule::PerconaProvider,
        check: Some(percona),
    },
    KindRule {
        api_version: "psmdb.percona.com/v1",
        kinds: &["PerconaServerMongoDBBackup"],
        provider: "percona-mongodb",
        missing: LintRule::PerconaBackupProvider,
        check: Some(percona_backup),
    },
    KindRule {
        api_version: "dragonflydb.io/v1alpha1",
        kinds: &["Dragonfly"],
        provider: "dragonfly-operator",
        missing: LintRule::DragonflyProvider,
        check: Some(dragonfly),
    },
    KindRule {
        api_version: "k8s.mariadb.com/v1alpha1",
        kinds: &["MariaDB", "Backup", "Database", "User", "Grant"],
        provider: "mariadb-operator",
        missing: LintRule::MariaDbProvider,
        check: Some(mariadb),
    },
    KindRule {
        api_version: "moco.cybozu.com/v1beta2",
        kinds: &["MySQLCluster"],
        provider: "moco",
        missing: LintRule::MocoProvider,
        check: Some(mysql_cluster),
    },
    KindRule {
        api_version: "traefik.io/v1alpha1",
        kinds: &["Middleware"],
        provider: "traefik",
        missing: LintRule::MiddlewareProvider,
        check: Some(middleware),
    },
];

impl KindRule {
    fn covers(&self, api_version: &str, kind: &str) -> bool {
        self.api_version == api_version && self.kinds.contains(&kind)
    }
}

/// Whether an operator rule covers the kind.
pub(super) fn covers(api_version: &str, kind: &str) -> bool {
    RULES.iter().any(|r| r.covers(api_version, kind))
}

/// Applies the operator rules that cover `doc`.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
pub(super) fn check(
    subject: &Subject<'_>,
    doc: &Value,
    head: &Head,
    docs: &[Value],
    at: &str,
    found: &mut Found,
) {
    let object = Object {
        subject,
        doc,
        head,
        docs,
        at,
    };
    for rule in &RULES {
        if !rule.covers(&head.api_version.0, &head.kind.0) {
            continue;
        }
        if !subject.requires(rule.provider) {
            found.push(at, rule.missing.clone());
        }
        if let Some(check) = rule.check {
            check(&object, found);
        }
    }
}

/// The `type` of a seccomp profile.
#[derive(Debug, Default, Deserialize)]
struct Typed {
    #[serde(default, rename = "type")]
    type_: Loose<String>,
}

/// A map with `enabled`.
#[derive(Debug, Default, Deserialize)]
struct Enabled {
    #[serde(default)]
    enabled: Loose<bool>,
}

/// A map with `name`.
#[derive(Debug, Default, Deserialize)]
struct Named {
    #[serde(default)]
    name: Loose<String>,
}

/// A reference to a key of a Secret.
#[derive(Debug, Default, Deserialize)]
struct KeyRef {
    #[serde(default)]
    name: Loose<String>,
    #[serde(default)]
    key: Loose<String>,
}

/// What a security context sets that Pod Security Admission does not check at restricted.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ContextView {
    #[serde(default)]
    sysctls: Loose<Vec<IgnoredAny>>,
    #[serde(default)]
    se_linux_options: Keys,
}

type PodContext = Obj<PodSecurityContext, ContextView>;
type ContainerContext = Obj<SecurityContext, ContextView>;

/// A map of resources, checked like a container's.
fn resources(resources: &Obj<ResourceRequirements>, at: &str, found: &mut Found) {
    match &resources.typed {
        Ok(r) => requirements(Some(r), at, found),
        Err(message) => {
            let resource = message.clone();
            found.push(at, LintRule::PositiveResource { resource });
        }
    }
}

/// Checks the security contexts that an operator copies into the pods it creates, as one
/// container named `operand` at restricted.
fn operand_context(pod: &PodContext, container: &ContainerContext, at: &str, found: &mut Found) {
    for view in [&pod.view, &container.view] {
        if !view.sysctls.0.is_empty() {
            found.push(at, LintRule::OperandSysctls);
        }
        if !view.se_linux_options.0.is_empty() {
            found.push(at, LintRule::OperandSeLinux);
        }
    }
    let unreadable = |found: &mut Found, refusal: &String| {
        let refusal = refusal.clone();
        found.push(
            at,
            LintRule::PodSecurity {
                level: None,
                refusal,
            },
        );
    };
    let context = match &container.typed {
        Ok(context) => context.clone(),
        Err(e) => return unreadable(found, e),
    };
    let mut spec = PodSpec {
        containers: vec![Container {
            name: "operand".to_owned(),
            security_context: Some(context),
            ..Container::default()
        }],
        ..PodSpec::default()
    };
    if pod.present {
        match &pod.typed {
            Ok(context) => spec.security_context = Some(context.clone()),
            Err(e) => return unreadable(found, e),
        }
    }
    pod_security(
        &ObjectMeta::default(),
        &spec,
        PodSecurity::Restricted,
        at,
        found,
    );
}

/// Requires the template label in `podMetadata`, which the operator copies to its pods.
fn pod_metadata(subject: &Subject<'_>, metadata: &Labelled, at: &str, found: &mut Found) {
    if metadata.label(TEMPLATE_LABEL) != subject.id {
        found.push(&format!("{at}/podMetadata"), LintRule::TemplateLabel);
    }
}

/// Refuses each key that is not in `allowed`, naming it after `what`.
fn only_keys(keys: &Keys, allowed: &[&str], what: &str, at: &str, found: &mut Found) {
    for key in keys.0.iter().filter(|k| !allowed.contains(&k.as_str())) {
        let field = format!("{what} {key}");
        found.push(at, LintRule::UnsupportedOverride { field });
    }
}

/// Whether `docs` declare the ServiceAccount `name` with its token disabled.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
fn declared_service_account(docs: &[Value], name: &str) -> bool {
    docs.iter().any(|doc| {
        view::<Head>(doc).is("v1", "ServiceAccount", name)
            && ServiceAccount::deserialize(doc)
                .is_ok_and(|a| a.automount_service_account_token == Some(false))
    })
}

/// Requires the ServiceAccount an operator's pods get to be declared with its token disabled.
fn service_account(object: &Object<'_>, name: &str, at: &str, found: &mut Found) {
    if !declared_service_account(object.docs, name) {
        found.push(at, LintRule::OperandServiceAccount);
    }
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CnpgCluster {
    #[serde(default)]
    image_name: Loose<String>,
    #[serde(default)]
    resources: Obj<ResourceRequirements>,
    #[serde(default)]
    inherited_metadata: Loose<Labelled>,
    #[serde(default)]
    seccomp_profile: Loose<Typed>,
    #[serde(default, rename = "postgresUID")]
    postgres_uid: Zero,
    #[serde(default, rename = "postgresGID")]
    postgres_gid: Zero,
    #[serde(default)]
    plugins: Loose<Vec<Loose<Named>>>,
}

fn cnpg_cluster(object: &Object<'_>, found: &mut Found) {
    let at = object.at;
    let spec = view::<Spec<Loose<CnpgCluster>>>(object.doc).spec.0;
    if !pinned(&spec.image_name.0) {
        found.push(at, LintRule::CnpgDigest);
    }
    resources(&spec.resources, at, found);
    if spec.inherited_metadata.0.label(TEMPLATE_LABEL) != object.subject.id {
        found.push(&format!("{at}/inheritedMetadata"), LintRule::TemplateLabel);
    }
    if !object.subject.has_risks() {
        found.push(at, LintRule::CnpgRisks);
    }
    if !matches!(
        spec.seccomp_profile.0.type_.0.as_str(),
        "RuntimeDefault" | "Localhost"
    ) {
        found.push(at, LintRule::CnpgSeccomp);
    }
    for id in [spec.postgres_uid, spec.postgres_gid] {
        if id.0 {
            found.push(at, LintRule::CnpgRoot);
        }
    }
    for plugin in &spec.plugins.0 {
        if plugin.0.name.0 != "barman-cloud.cloudnative-pg.io" {
            found.push(at, LintRule::CnpgPlugin);
        }
    }
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ObjectStore {
    #[serde(default)]
    instance_sidecar_configuration: Loose<Resourced>,
}

/// A map with `resources`.
#[derive(Debug, Default, Deserialize)]
struct Resourced {
    #[serde(default)]
    resources: Obj<ResourceRequirements>,
}

fn barman_object_store(object: &Object<'_>, found: &mut Found) {
    let spec = view::<Spec<Loose<ObjectStore>>>(object.doc).spec.0;
    let at = format!("{}/barman-sidecar", object.at);
    resources(&spec.instance_sidecar_configuration.0.resources, &at, found);
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Percona {
    #[serde(default)]
    image: Loose<String>,
    #[serde(default)]
    init_image: Loose<String>,
    #[serde(default)]
    backup: Loose<PerconaPart>,
    #[serde(default)]
    sharding: Loose<Enabled>,
    #[serde(default)]
    unsafe_flags: Loose<BTreeMap<String, Loose<bool>>>,
    #[serde(default)]
    init_container_security_context: ContainerContext,
    #[serde(default)]
    replsets: Loose<Vec<Loose<PerconaPart>>>,
}

/// A replica set of a Percona MongoDB cluster, or its backup, which runs as an operand too.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PerconaPart {
    #[serde(default)]
    enabled: Loose<bool>,
    #[serde(default)]
    image: Loose<String>,
    #[serde(default)]
    storages: Loose<BTreeMap<String, Loose<PerconaStorage>>>,
    #[serde(default)]
    labels: Labels,
    #[serde(default)]
    arbiter: Loose<Enabled>,
    #[serde(default)]
    nonvoting: Loose<Enabled>,
    #[serde(default)]
    hidden: Loose<Enabled>,
    #[serde(default)]
    resources: Obj<ResourceRequirements>,
    #[serde(default)]
    pod_security_context: PodContext,
    #[serde(default)]
    container_security_context: ContainerContext,
    #[serde(default)]
    service_account_name: Loose<String>,
    #[serde(default)]
    volume_spec: Keys,
    #[serde(default)]
    sidecars: Loose<Vec<IgnoredAny>>,
    #[serde(default)]
    sidecar_volumes: Loose<Vec<IgnoredAny>>,
    #[serde(default)]
    host_aliases: Loose<Vec<IgnoredAny>>,
    #[serde(default)]
    init_containers: Loose<Vec<IgnoredAny>>,
}

#[derive(Debug, Default, Deserialize)]
struct PerconaStorage {
    #[serde(default, rename = "type")]
    type_: Loose<String>,
    #[serde(default)]
    s3: Loose<PerconaS3>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PerconaS3 {
    #[serde(default)]
    credentials_secret: Loose<String>,
    #[serde(default)]
    endpoint_url: Loose<String>,
    #[serde(default, rename = "insecureSkipTLSVerify")]
    insecure_skip_tls_verify: Loose<bool>,
}

impl PerconaStorage {
    /// S3 over HTTPS with verified TLS and credentials from a Secret.
    fn is_safe(&self) -> bool {
        let s3 = &self.s3.0;
        self.type_.0 == "s3"
            && !s3.credentials_secret.0.is_empty()
            && s3.endpoint_url.0.starts_with("https://")
            && !s3.insecure_skip_tls_verify.0
    }
}

fn percona(object: &Object<'_>, found: &mut Found) {
    let at = object.at;
    if !object.subject.has_risks() {
        found.push(at, LintRule::PerconaRisks);
    }
    let spec = view::<Spec<Loose<Percona>>>(object.doc).spec.0;
    let backup = &spec.backup.0;
    let mut images = vec![("image", &spec.image.0), ("initImage", &spec.init_image.0)];
    if backup.enabled.0 {
        images.push(("backup.image", &backup.image.0));
        for (name, storage) in &backup.storages.0 {
            if !storage.0.is_safe() {
                let storage = Some(name.clone());
                found.push(at, LintRule::PerconaBackupStorage { storage });
            }
        }
    }
    for (field, image) in images {
        if !pinned(image) {
            let field = field.to_owned();
            found.push(at, LintRule::PerconaDigest { field });
        }
    }
    if spec.sharding.0.enabled.0 {
        found.push(at, LintRule::PerconaSharding);
    }
    for (flag, set) in &spec.unsafe_flags.0 {
        if set.0 && flag != "replsetSize" {
            let flag = flag.clone();
            found.push(at, LintRule::PerconaUnsafeFlag { flag });
        }
    }
    let init = format!("{at}/init");
    operand_context(
        &PodContext::default(),
        &spec.init_container_security_context,
        &init,
        found,
    );
    let replsets = &spec.replsets.0;
    if replsets.is_empty() {
        found.push(at, LintRule::PerconaNoReplicaSet);
    }
    let backup = backup.enabled.0.then_some(backup);
    for (n, part) in replsets.iter().map(|p| &p.0).chain(backup).enumerate() {
        let at = format!("{at}/operand-{n}");
        if n < replsets.len() {
            if part.labels.label(TEMPLATE_LABEL) != object.subject.id {
                found.push(&at, LintRule::TemplateLabel);
            }
            let members = [
                ("arbiter", &part.arbiter),
                ("nonvoting", &part.nonvoting),
                ("hidden", &part.hidden),
            ];
            for (field, member) in members {
                if member.0.enabled.0 {
                    let field = field.to_owned();
                    found.push(&at, LintRule::UnsupportedOverride { field });
                }
            }
        }
        resources(&part.resources, &at, found);
        operand_context(
            &part.pod_security_context,
            &part.container_security_context,
            &at,
            found,
        );
        service_account(object, &part.service_account_name.0, &at, found);
        for key in &part.volume_spec.0 {
            if key != "persistentVolumeClaim" {
                found.push(&at, LintRule::PerconaVolume);
            }
        }
        let lists = [
            ("sidecars", &part.sidecars),
            ("sidecarVolumes", &part.sidecar_volumes),
            ("hostAliases", &part.host_aliases),
            ("initContainers", &part.init_containers),
        ];
        for (field, list) in lists {
            if !list.0.is_empty() {
                let field = field.to_owned();
                found.push(&at, LintRule::UnsupportedOverride { field });
            }
        }
    }
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PerconaBackup {
    #[serde(default)]
    cluster_name: Loose<String>,
    #[serde(default)]
    storage_name: Loose<String>,
    #[serde(default, rename = "type")]
    type_: Loose<String>,
    #[serde(default)]
    compression_type: Loose<String>,
}

fn percona_backup(object: &Object<'_>, found: &mut Found) {
    let at = object.at;
    let keys = view::<Spec<Keys>>(object.doc).spec;
    let allowed = [
        "clusterName",
        "storageName",
        "type",
        "compressionType",
        "startingDeadlineSeconds",
    ];
    for key in keys.0.iter().filter(|k| !allowed.contains(&k.as_str())) {
        let field = key.clone();
        found.push(at, LintRule::PerconaBackupOverride { field });
    }
    let spec = view::<Spec<Loose<PerconaBackup>>>(object.doc).spec.0;
    if spec.type_.0 != "logical" || spec.compression_type.0 != "gzip" {
        found.push(at, LintRule::PerconaBackupMode);
    }
    let (cluster, storage) = (&spec.cluster_name.0, &spec.storage_name.0);
    if cluster.is_empty() || storage.is_empty() {
        found.push(at, LintRule::PerconaBackupTarget);
        return;
    }
    let declared = object
        .docs
        .iter()
        .find(|doc| view::<Head>(doc).is("psmdb.percona.com/v1", "PerconaServerMongoDB", cluster));
    let Some(declared) = declared else {
        found.push(at, LintRule::PerconaBackupCluster);
        return;
    };
    let backup = view::<Spec<Loose<Percona>>>(declared).spec.0.backup.0;
    let safe = backup
        .storages
        .0
        .get(storage)
        .is_some_and(|s| s.0.is_safe());
    if !backup.enabled.0 || !safe {
        found.push(at, LintRule::PerconaBackupStorage { storage: None });
    }
}

/// A Middleware whose `ipAllowList` is set to anything.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MiddlewareSet {
    #[serde(default)]
    ip_allow_list: Set,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Middleware {
    #[serde(default)]
    ip_allow_list: Obj<IgnoredAny, AllowList>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct AllowList {
    #[serde(default)]
    source_range: Loose<Vec<Loose<String>>>,
}

fn middleware(object: &Object<'_>, found: &mut Found) {
    let at = object.at;
    let keys = view::<Spec<Keys>>(object.doc).spec;
    let set = view::<Spec<Loose<MiddlewareSet>>>(object.doc).spec.0;
    if keys.0.len() != 1 || !set.ip_allow_list.0 {
        found.push(at, LintRule::IpAllowListOnly);
        return;
    }
    let allow = view::<Spec<Loose<Middleware>>>(object.doc)
        .spec
        .0
        .ip_allow_list;
    if allow.keys.0.len() != 1 {
        found.push(at, LintRule::IpAllowListKeys);
    }
    let ranges = &allow.view.source_range.0;
    if ranges.is_empty() {
        found.push(at, LintRule::IpAllowListEmpty);
    }
    for range in ranges {
        if prefix_bits(&range.0).is_none_or(|bits| bits == 0) {
            found.push(at, LintRule::IpAllowListCidr);
        }
    }
}

/// The prefix length of a CIDR, as Go's `netip.ParsePrefix` reads it: an IP address without a
/// zone, `/`, then a decimal length without a sign or leading zero that fits the address.
fn prefix_bits(cidr: &str) -> Option<u8> {
    let (address, bits) = cidr.rsplit_once('/')?;
    let address: IpAddr = address.parse().ok()?;
    let max = if address.is_ipv4() { 32 } else { 128 };
    if bits.is_empty()
        || !bits.bytes().all(|b| b.is_ascii_digit())
        || (bits.len() > 1 && bits.starts_with('0'))
    {
        return None;
    }
    bits.parse::<u8>().ok().filter(|b| *b <= max)
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Dragonfly {
    #[serde(default)]
    image: Loose<String>,
    #[serde(default)]
    pod_metadata: Loose<Labelled>,
    #[serde(default)]
    resources: Obj<ResourceRequirements>,
    #[serde(default)]
    pod_security_context: PodContext,
    #[serde(default)]
    container_security_context: ContainerContext,
    #[serde(default)]
    service_account_name: Loose<String>,
    #[serde(default)]
    authentication: Loose<DragonflyAuthentication>,
    #[serde(default)]
    args: Loose<Vec<Loose<String>>>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DragonflyAuthentication {
    #[serde(default)]
    password_from_secret: Loose<KeyRef>,
}

fn dragonfly(object: &Object<'_>, found: &mut Found) {
    let at = object.at;
    if !object.subject.has_risks() {
        found.push(at, LintRule::DragonflyRisks);
    }
    let keys = view::<Spec<Keys>>(object.doc).spec;
    let allowed = [
        "replicas",
        "image",
        "imagePullPolicy",
        "imagePullSecrets",
        "serviceAccountName",
        "args",
        "authentication",
        "env",
        "podSecurityContext",
        "containerSecurityContext",
        "podMetadata",
        "resources",
        "snapshot",
        "tlsSecretRef",
        "affinity",
        "nodeSelector",
        "tolerations",
        "topologySpreadConstraints",
    ];
    only_keys(&keys, &allowed, "Dragonfly", at, found);
    let spec = view::<Spec<Loose<Dragonfly>>>(object.doc).spec.0;
    if !pinned(&spec.image.0) {
        found.push(at, LintRule::DragonflyDigest);
    }
    pod_metadata(object.subject, &spec.pod_metadata.0, at, found);
    resources(&spec.resources, at, found);
    operand_context(
        &spec.pod_security_context,
        &spec.container_security_context,
        at,
        found,
    );
    service_account(object, &spec.service_account_name.0, at, found);
    let password = &spec.authentication.0.password_from_secret.0;
    if password.name.0.is_empty() || password.key.0.is_empty() {
        found.push(at, LintRule::DragonflyPassword);
    }
    // These flags open a port or the admin console without a password.
    for arg in &spec.args.0 {
        for flag in ["--admin_nopass", "--memcached_port"] {
            if arg.0 == flag || arg.0.strip_prefix(flag).is_some_and(|r| r.starts_with('=')) {
                let flag = flag.to_owned();
                found.push(at, LintRule::DragonflyFlag { flag });
            }
        }
    }
}

/// The fields of the MariaDB Operator's kinds that the lint reads.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MariaDb {
    #[serde(default)]
    #[serde(rename = "mariaDbRef")]
    reference: Obj<IgnoredAny, MariaDbRef>,
    #[serde(default)]
    image: Loose<String>,
    #[serde(default)]
    pod_metadata: Loose<Labelled>,
    #[serde(default)]
    resources: Obj<ResourceRequirements>,
    #[serde(default)]
    pod_security_context: PodContext,
    #[serde(default)]
    security_context: ContainerContext,
    #[serde(default)]
    service_account_name: Loose<String>,
    #[serde(default)]
    staging_storage: Keys,
    #[serde(default)]
    storage: Obj<IgnoredAny, MariaDbStorage>,
    #[serde(default)]
    password_secret_key_ref: Loose<KeyRef>,
    #[serde(default)]
    database: Loose<String>,
    #[serde(default)]
    grant_option: Loose<bool>,
}

#[derive(Debug, Default, Deserialize)]
struct MariaDbRef {
    #[serde(default)]
    name: Loose<String>,
    #[serde(default)]
    namespace: Loose<String>,
}

#[derive(Debug, Default, Deserialize)]
struct MariaDbStorage {
    #[serde(default)]
    s3: Loose<MariaDbS3>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MariaDbS3 {
    #[serde(default)]
    bucket: Loose<String>,
    #[serde(default)]
    endpoint: Loose<String>,
    #[serde(default)]
    access_key_id_secret_key_ref: Keys,
    #[serde(default)]
    secret_access_key_secret_key_ref: Keys,
    #[serde(default)]
    tls: Loose<Enabled>,
}

/// Requires `mariaDbRef` to name a MariaDB of this template. The operator is cluster-wide, so a
/// reference to another namespace reaches another tenant.
fn mariadb_ref(object: &Object<'_>, spec: &MariaDb, found: &mut Found) {
    let at = object.at;
    let reference = &spec.reference;
    let allowed = ["name", "namespace", "waitForIt"];
    only_keys(&reference.keys, &allowed, "mariaDbRef", at, found);
    let namespace = &reference.view.namespace.0;
    if !namespace.is_empty() && namespace != object.subject.namespace {
        found.push(at, LintRule::MariaDbRefNamespace);
    }
    let name = &reference.view.name.0;
    let declared = object
        .docs
        .iter()
        .any(|doc| view::<Head>(doc).is("k8s.mariadb.com/v1alpha1", "MariaDB", name));
    if !declared {
        found.push(at, LintRule::MariaDbRefUndeclared);
    }
}

/// The pod of an operand the MariaDB Operator creates, for a MariaDB or a Backup.
fn mariadb_operand(object: &Object<'_>, spec: &MariaDb, found: &mut Found) {
    let at = object.at;
    pod_metadata(object.subject, &spec.pod_metadata.0, at, found);
    resources(&spec.resources, at, found);
    operand_context(
        &spec.pod_security_context,
        &spec.security_context,
        at,
        found,
    );
    service_account(object, &spec.service_account_name.0, at, found);
}

fn mariadb(object: &Object<'_>, found: &mut Found) {
    let at = object.at;
    let keys = view::<Spec<Keys>>(object.doc).spec;
    let spec = view::<Spec<Loose<MariaDb>>>(object.doc).spec.0;
    let common = [
        "mariaDbRef",
        "cleanupPolicy",
        "requeueInterval",
        "retryInterval",
    ];
    match object.head.kind.0.as_str() {
        "MariaDB" => {
            if !object.subject.has_risks() {
                found.push(at, LintRule::MariaDbRisks);
            }
            let allowed = [
                "image",
                "imagePullPolicy",
                "imagePullSecrets",
                "replicas",
                "rootPasswordSecretKeyRef",
                "database",
                "username",
                "passwordSecretKeyRef",
                "serviceAccountName",
                "tls",
                "storage",
                "podMetadata",
                "podSecurityContext",
                "securityContext",
                "resources",
                "updateStrategy",
                "timeZone",
                "affinity",
                "nodeSelector",
                "tolerations",
                "topologySpreadConstraints",
            ];
            only_keys(&keys, &allowed, "MariaDB", at, found);
            if !pinned(&spec.image.0) {
                found.push(at, LintRule::MariaDbDigest);
            }
            mariadb_operand(object, &spec, found);
        }
        "Backup" => mariadb_backup(object, &keys, &spec, found),
        "Database" => {
            mariadb_ref(object, &spec, found);
            let allowed = [&["name", "characterSet", "collate"][..], &common].concat();
            only_keys(&keys, &allowed, "Database", at, found);
        }
        "User" => {
            mariadb_ref(object, &spec, found);
            let own = [
                "name",
                "host",
                "passwordSecretKeyRef",
                "require",
                "maxUserConnections",
            ];
            let allowed = [&own[..], &common].concat();
            only_keys(&keys, &allowed, "User", at, found);
            let password = &spec.password_secret_key_ref.0;
            if password.name.0.is_empty() || password.key.0.is_empty() {
                found.push(at, LintRule::MariaDbUserPassword);
            }
        }
        "Grant" => {
            mariadb_ref(object, &spec, found);
            let own = [
                "username",
                "host",
                "database",
                "table",
                "privileges",
                "grantOption",
            ];
            let allowed = [&own[..], &common].concat();
            only_keys(&keys, &allowed, "Grant", at, found);
            if matches!(spec.database.0.as_str(), "" | "*") {
                found.push(at, LintRule::GlobalGrant);
            }
            if spec.grant_option.0 {
                found.push(at, LintRule::GrantOption);
            }
        }
        _ => {}
    }
}

/// Checks a MariaDB Operator `Backup`: its fields, its operand and its S3 store.
fn mariadb_backup(object: &Object<'_>, keys: &Keys, spec: &MariaDb, found: &mut Found) {
    let at = object.at;
    mariadb_ref(object, spec, found);
    let allowed = [
        "mariaDbRef",
        "schedule",
        "timeZone",
        "successfulJobsHistoryLimit",
        "failedJobsHistoryLimit",
        "serviceAccountName",
        "podMetadata",
        "podSecurityContext",
        "securityContext",
        "resources",
        "stagingStorage",
        "storage",
        "databases",
        "maxRetention",
        "compression",
        "backoffLimit",
    ];
    only_keys(keys, &allowed, "backup", at, found);
    mariadb_operand(object, spec, found);
    for key in &spec.staging_storage.0 {
        if key != "persistentVolumeClaim" {
            found.push(at, LintRule::MariaDbStagingStorage);
        }
    }
    let storage = &spec.storage;
    let s3 = &storage.view.s3.0;
    if storage.keys.0.len() != 1
        || s3.bucket.0.is_empty()
        || s3.endpoint.0.is_empty()
        || s3.access_key_id_secret_key_ref.0.is_empty()
        || s3.secret_access_key_secret_key_ref.0.is_empty()
        || !s3.tls.0.enabled.0
    {
        found.push(at, LintRule::MariaDbBackupS3);
    }
}

/// The containers MOCO adds to each MySQL pod, which `overwriteContainers` may change.
/// <https://github.com/cybozu-go/moco/blob/v0.37.0/docs/customize-system-container.md>
const MOCO_CONTAINERS: [&str; 4] = ["agent", "copy-moco-init", "moco-init", "slow-log"];

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MySqlCluster {
    #[serde(default)]
    pod_template: Obj<IgnoredAny, MocoTemplate>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MocoTemplate {
    #[serde(default)]
    metadata: Loose<Labelled>,
    #[serde(default)]
    spec: Obj<PodSpec, MocoPod>,
    #[serde(default)]
    overwrite_containers: Loose<Vec<Obj<Container, Named>>>,
}

#[derive(Debug, Default, Deserialize)]
struct MocoPod {
    #[serde(default)]
    containers: Loose<Vec<Keys>>,
}

/// Checks a MOCO `MySQLCluster`: the mysqld container in `spec.podTemplate` and the security
/// contexts of the containers MOCO adds.
fn mysql_cluster(object: &Object<'_>, found: &mut Found) {
    let at = object.at;
    if !object.subject.has_risks() {
        found.push(at, LintRule::MocoRisks);
    }
    let keys = view::<Spec<Keys>>(object.doc).spec;
    let allowed = [
        "replicas",
        "podTemplate",
        "volumeClaimTemplates",
        "mysqlConfigMapName",
        "maxDelaySeconds",
        "startupWaitSeconds",
        "disableSlowQueryLogContainer",
        "logRotationSchedule",
    ];
    only_keys(&keys, &allowed, "MySQLCluster", at, found);
    let template = view::<Spec<Loose<MySqlCluster>>>(object.doc)
        .spec
        .0
        .pod_template;
    let allowed = ["metadata", "spec", "overwriteContainers"];
    only_keys(&template.keys, &allowed, "podTemplate", at, found);
    let template = template.view;
    if template.metadata.0.label(TEMPLATE_LABEL) != object.subject.id {
        found.push(&format!("{at}/podTemplate"), LintRule::TemplateLabel);
    }
    let allowed = [
        "securityContext",
        "containers",
        "affinity",
        "nodeSelector",
        "tolerations",
        "topologySpreadConstraints",
        "terminationGracePeriodSeconds",
    ];
    only_keys(&template.spec.keys, &allowed, "podTemplate.spec", at, found);
    let mut pod = match template.spec.typed {
        Ok(pod) => pod,
        Err(refusal) => {
            found.push(
                at,
                LintRule::PodSecurity {
                    level: None,
                    refusal,
                },
            );
            return;
        }
    };
    if pod.containers.len() != 1 || pod.containers.first().is_none_or(|c| c.name != "mysqld") {
        let field = "containers other than mysqld".to_owned();
        found.push(at, LintRule::UnsupportedOverride { field });
    }
    let allowed = ["name", "image", "resources", "securityContext"];
    for container in &template.spec.view.containers.0 {
        only_keys(container, &allowed, "container", at, found);
    }
    for container in &pod.containers {
        let at = format!("{at}/{}", container.name);
        if !pinned(container.image.as_deref().unwrap_or_default()) {
            found.push(&at, LintRule::ImageDigest);
        }
        requirements(container.resources.as_ref(), &at, found);
    }
    for overwrite in &template.overwrite_containers.0 {
        match &overwrite.typed {
            Ok(added) if MOCO_CONTAINERS.contains(&added.name.as_str()) => {
                pod.containers.push(Container {
                    name: added.name.clone(),
                    security_context: added.security_context.clone(),
                    ..Container::default()
                });
            }
            Ok(_) | Err(_) => {
                let field = format!("overwriteContainers {}", overwrite.view.name.0);
                found.push(at, LintRule::UnsupportedOverride { field });
            }
        }
    }
    // A container MOCO adds without an overwrite runs with MOCO's default security context,
    // which restricted does not admit.
    for name in MOCO_CONTAINERS {
        if !pod.containers.iter().any(|c| c.name == name) {
            pod.containers.push(Container {
                name: name.to_owned(),
                ..Container::default()
            });
        }
    }
    pod_security(
        &ObjectMeta::default(),
        &pod,
        PodSecurity::Restricted,
        at,
        found,
    );
}

#[cfg(test)]
mod tests {
    use serde_json::json;
    use zeabur_catalog_format::problem::LintRule;

    use super::super::Found;
    use super::super::tests::{
        SAFE, assert_breaks, digest, replace, safety, subject, template, with_risk,
    };
    use super::super::view::{Head, view};
    use super::Object;

    #[test]
    fn cnpg_does_not_borrow_another_provider() {
        let manifest = format!(
            "{SAFE}---\napiVersion: postgresql.cnpg.io/v1\nkind: Cluster\nmetadata: {{name: database}}\n\
             spec:\n  inheritedMetadata: {{labels: {{templates.zeabur.com/template: app}}}}\n  \
             imageName: ghcr.io/example/postgres{}\n  seccompProfile: {{type: RuntimeDefault}}\n  \
             resources:\n    requests: {{cpu: 10m, memory: 16Mi}}\n    limits: {{cpu: 100m, memory: 32Mi}}\n",
            digest()
        );
        let traefik = with_risk(template(&[], &["traefik"]));
        assert_breaks!(safety(&traefik, &manifest), LintRule::CnpgProvider);
        let cnpg = with_risk(template(&[], &["cloudnative-pg"]));
        assert_eq!(safety(&cnpg, &manifest), []);
        let unlabelled = replace(
            &manifest,
            "  inheritedMetadata: {labels: {templates.zeabur.com/template: app}}\n",
            "",
        );
        assert_breaks!(safety(&cnpg, &unlabelled), LintRule::TemplateLabel);
    }

    #[test]
    fn a_middleware_is_an_explicit_ip_allow_list_only() {
        let app = template(&[], &["traefik"]);
        let prefix = format!(
            "{SAFE}---\napiVersion: traefik.io/v1alpha1\nkind: Middleware\nmetadata: {{name: admin}}\nspec:\n"
        );
        let allowed = "  ipAllowList: {sourceRange: [192.0.2.0/24]}\n";
        assert_eq!(safety(&app, &(prefix.clone() + allowed)), []);
        let rules = safety(
            &app,
            &(prefix.clone() + "  ipAllowList: {sourceRange: [0.0.0.0/0]}\n"),
        );
        assert_breaks!(rules, LintRule::IpAllowListCidr);
        let rules = safety(
            &app,
            &(prefix.clone() + "  forwardAuth: {address: 'http://another-namespace'}\n"),
        );
        assert_breaks!(rules, LintRule::IpAllowListOnly);
        let rules = safety(
            &app,
            &(prefix + allowed + "  stripPrefix: {prefixes: [/]}\n"),
        );
        assert_breaks!(rules, LintRule::IpAllowListOnly);
    }

    #[test]
    fn an_ingress_requires_its_runtime_providers() {
        let manifest = format!(
            "{SAFE}---\napiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: app\n  \
             annotations: {{cert-manager.io/cluster-issuer: issuer}}\nspec:\n  ingressClassName: traefik\n"
        );
        assert_eq!(
            safety(&template(&[], &["traefik", "cert-manager"]), &manifest),
            []
        );
        let rules = safety(&template(&[], &["traefik"]), &manifest);
        assert_breaks!(rules, LintRule::ClusterIssuerProvider);
        let rules = safety(&template(&[], &["cert-manager"]), &manifest);
        assert_breaks!(rules, LintRule::TraefikProvider);
    }

    const OPERAND: &str = "
  serviceAccountName: db
  resources:
    requests: {cpu: 10m, memory: 16Mi}
    limits: {cpu: 100m, memory: 32Mi}
  podSecurityContext: {runAsNonRoot: true, runAsUser: 999, seccompProfile: {type: RuntimeDefault}}
";

    const ACCOUNT: &str = "---
apiVersion: v1
kind: ServiceAccount
metadata: {name: db}
automountServiceAccountToken: false
";

    fn dragonfly() -> String {
        format!(
            "{SAFE}{ACCOUNT}---
apiVersion: dragonflydb.io/v1alpha1
kind: Dragonfly
metadata: {{name: queue}}
spec:
  image: docker.dragonflydb.io/dragonflydb/dragonfly{}
  args: [--cache_mode=false]
  podMetadata: {{labels: {{templates.zeabur.com/template: app}}}}
  authentication: {{passwordFromSecret: {{name: app, key: password}}}}
  containerSecurityContext: {{allowPrivilegeEscalation: false, capabilities: {{drop: [ALL]}}}}{OPERAND}",
            digest()
        )
    }

    fn mariadb() -> String {
        format!(
            "{SAFE}{ACCOUNT}---
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata: {{name: mariadb}}
spec:
  image: docker.io/mariadb{}
  podMetadata: {{labels: {{templates.zeabur.com/template: app}}}}
  securityContext: {{allowPrivilegeEscalation: false, capabilities: {{drop: [ALL]}}}}{OPERAND}---
apiVersion: k8s.mariadb.com/v1alpha1
kind: Grant
metadata: {{name: app}}
spec:
  mariaDbRef: {{name: mariadb}}
  username: app
  database: app
  privileges: [SELECT]
---
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata: {{name: daily}}
spec:
  mariaDbRef: {{name: mariadb}}
  podMetadata: {{labels: {{templates.zeabur.com/template: app}}}}
  securityContext: {{allowPrivilegeEscalation: false, capabilities: {{drop: [ALL]}}}}{OPERAND}  storage:
    s3:
      bucket: backups
      endpoint: s3.example.test:443
      accessKeyIdSecretKeyRef: {{name: s3, key: id}}
      secretAccessKeySecretKeyRef: {{name: s3, key: secret}}
      tls: {{enabled: true}}
",
            digest()
        )
    }

    fn mysql() -> String {
        format!(
            "{SAFE}---
apiVersion: moco.cybozu.com/v1beta2
kind: MySQLCluster
metadata: {{name: mysql}}
spec:
  replicas: 1
  podTemplate:
    metadata: {{labels: {{templates.zeabur.com/template: app}}}}
    spec:
      securityContext: {{runAsNonRoot: true, runAsUser: 10000, seccompProfile: {{type: RuntimeDefault}}}}
      containers:
        - name: mysqld
          image: ghcr.io/cybozu-go/moco/mysql{}
          resources:
            requests: {{cpu: 10m, memory: 16Mi}}
            limits: {{cpu: 100m, memory: 32Mi}}
          securityContext: &system {{allowPrivilegeEscalation: false, capabilities: {{drop: [ALL]}}}}
    overwriteContainers:
      - {{name: agent, securityContext: *system}}
      - {{name: moco-init, securityContext: *system}}
      - {{name: copy-moco-init, securityContext: *system}}
      - {{name: slow-log, securityContext: *system}}
  volumeClaimTemplates:
    - metadata: {{name: mysql-data}}
      spec: {{accessModes: [ReadWriteOnce], resources: {{requests: {{storage: 1Gi}}}}}}
",
            digest()
        )
    }

    /// The rules `manifest` breaks for a template with a risk that requires `provider`.
    fn database(manifest: &str, provider: &str) -> Vec<LintRule> {
        safety(&with_risk(template(&[], &[provider])), manifest)
    }

    #[test]
    fn mysql_stays_in_the_template() {
        let (mysql, digest) = (mysql(), digest());
        assert_eq!(database(&mysql, "moco"), []);
        assert_breaks!(database(&mysql, "mariadb-operator"), LintRule::MocoProvider);
        let tag = replace(&mysql, &format!("mysql{digest}"), "mysql:8.4.10");
        assert_breaks!(database(&tag, "moco"), LintRule::ImageDigest);
        let label = replace(
            &mysql,
            "    metadata: {labels: {templates.zeabur.com/template: app}}\n",
            "",
        );
        assert_breaks!(database(&label, "moco"), LintRule::TemplateLabel);
        let root = replace(&mysql, "runAsUser: 10000", "runAsUser: 0");
        assert_breaks!(database(&root, "moco"), LintRule::PodSecurity { .. });
        let system = replace(
            &mysql,
            "      - {name: slow-log, securityContext: *system}\n",
            "",
        );
        assert_breaks!(database(&system, "moco"), LintRule::PodSecurity { .. });
        let unbounded = replace(
            &mysql,
            "            limits: {cpu: 100m, memory: 32Mi}\n",
            "",
        );
        assert_breaks!(
            database(&unbounded, "moco"),
            LintRule::PositiveResource { .. }
        );
        let service = mysql.clone() + "  primaryServiceTemplate: {spec: {type: LoadBalancer}}\n";
        assert_breaks!(
            database(&service, "moco"),
            LintRule::UnsupportedOverride { .. }
        );
        let sidecar = replace(
            &mysql,
            "        - name: mysqld\n",
            &format!("        - {{name: shell, image: busybox{digest}}}\n        - name: mysqld\n"),
        );
        assert_breaks!(
            database(&sidecar, "moco"),
            LintRule::UnsupportedOverride { .. }
        );
    }

    #[test]
    fn dragonfly_stays_in_the_template() {
        let (dragonfly, digest) = (dragonfly(), digest());
        let provider = "dragonfly-operator";
        assert_eq!(database(&dragonfly, provider), []);
        assert_breaks!(
            database(&dragonfly, "mariadb-operator"),
            LintRule::DragonflyProvider
        );
        let tag = replace(&dragonfly, &format!("dragonfly{digest}"), "dragonfly:v1");
        assert_breaks!(database(&tag, provider), LintRule::DragonflyDigest);
        let open = replace(
            &dragonfly,
            "  authentication: {passwordFromSecret: {name: app, key: password}}\n",
            "",
        );
        assert_breaks!(database(&open, provider), LintRule::DragonflyPassword);
        let memcached = replace(&dragonfly, "--cache_mode=false", "--memcached_port=11211");
        assert_breaks!(
            database(&memcached, provider),
            LintRule::DragonflyFlag { .. }
        );
        let label = replace(
            &dragonfly,
            "  podMetadata: {labels: {templates.zeabur.com/template: app}}\n",
            "",
        );
        assert_breaks!(database(&label, provider), LintRule::TemplateLabel);
        let sidecar = dragonfly + "  additionalContainers: [{name: x, image: x}]\n";
        assert_breaks!(
            database(&sidecar, provider),
            LintRule::UnsupportedOverride { .. }
        );
    }

    #[test]
    fn mariadb_stays_in_the_template() {
        let mariadb = mariadb();
        let provider = "mariadb-operator";
        assert_eq!(database(&mariadb, provider), []);
        assert_breaks!(
            database(&mariadb, "dragonfly-operator"),
            LintRule::MariaDbProvider
        );
        let root = replace(&mariadb, "runAsUser: 999", "runAsUser: 0");
        assert_breaks!(database(&root, provider), LintRule::PodSecurity { .. });
        let config = replace(
            &mariadb,
            "kind: MariaDB\nmetadata: {name: mariadb}\nspec:\n",
            "kind: MariaDB\nmetadata: {name: mariadb}\nspec:\n  myCnf: skip-grant-tables\n",
        );
        assert_breaks!(
            database(&config, provider),
            LintRule::UnsupportedOverride { .. }
        );
        let other = replace(
            &mariadb,
            "mariaDbRef: {name: mariadb}",
            "mariaDbRef: {name: mariadb, namespace: other}",
        );
        assert_breaks!(database(&other, provider), LintRule::MariaDbRefNamespace);
        let shared = replace(
            &mariadb,
            "mariaDbRef: {name: mariadb}",
            "mariaDbRef: {name: shared}",
        );
        assert_breaks!(database(&shared, provider), LintRule::MariaDbRefUndeclared);
        let global = replace(&mariadb, "database: app", "database: '*'");
        assert_breaks!(database(&global, provider), LintRule::GlobalGrant);
        let plain = replace(&mariadb, "tls: {enabled: true}", "tls: {enabled: false}");
        assert_breaks!(database(&plain, provider), LintRule::MariaDbBackupS3);
    }

    /// A Percona backup of the cluster `cluster` to `storage`, and the cluster `database` whose
    /// backup storage `s3` uses the Secret `credentials`.
    #[derive(Clone, Copy)]
    struct Backup<'a> {
        provider: &'a str,
        cluster: &'a str,
        storage: &'a str,
        mode: &'a str,
        credentials: &'a str,
        insecure: bool,
        plain_http: bool,
        options: bool,
    }

    const DECLARED: Backup<'static> = Backup {
        provider: "percona-mongodb",
        cluster: "database",
        storage: "s3",
        mode: "logical",
        credentials: "backup-credentials",
        insecure: false,
        plain_http: false,
        options: false,
    };

    fn backup(case: Backup<'_>) -> Vec<LintRule> {
        let template = template(&[], &[case.provider]);
        let mut doc = json!({
            "apiVersion": "psmdb.percona.com/v1",
            "kind": "PerconaServerMongoDBBackup",
            "spec": {
                "clusterName": case.cluster,
                "storageName": case.storage,
                "type": case.mode,
                "compressionType": "gzip",
            },
        });
        if case.options {
            doc["spec"]["backupOptions"] = json!({"args": ["--insecure"]});
        }
        let scheme = if case.plain_http { "http" } else { "https" };
        let cluster = json!({
            "apiVersion": "psmdb.percona.com/v1",
            "kind": "PerconaServerMongoDB",
            "metadata": {"name": "database"},
            "spec": {"backup": {"enabled": true, "storages": {"s3": {"type": "s3", "s3": {
                "credentialsSecret": case.credentials,
                "endpointUrl": format!("{scheme}://s3.example.test"),
                "insecureSkipTLSVerify": case.insecure,
            }}}}},
        });
        let mut found = Found::default();
        let head: Head = view(&doc);
        let docs = [cluster];
        super::check(
            &subject(&template),
            &doc,
            &head,
            &docs,
            "backup/initial",
            &mut found,
        );
        found.0.into_iter().map(|p| p.rule).collect()
    }

    #[test]
    fn a_percona_backup_stays_in_its_declared_cluster_and_storage() {
        assert_eq!(backup(DECLARED), []);
        let rules = backup(Backup {
            cluster: "other",
            ..DECLARED
        });
        assert_breaks!(rules, LintRule::PerconaBackupCluster);
        let rules = backup(Backup {
            storage: "other",
            ..DECLARED
        });
        assert_breaks!(rules, LintRule::PerconaBackupStorage { .. });
        let rules = backup(Backup {
            credentials: "",
            ..DECLARED
        });
        assert_breaks!(rules, LintRule::PerconaBackupStorage { .. });
        let rules = backup(Backup {
            provider: "cloudnative-pg",
            ..DECLARED
        });
        assert_breaks!(rules, LintRule::PerconaBackupProvider);
        let rules = backup(Backup {
            mode: "incremental",
            ..DECLARED
        });
        assert_breaks!(rules, LintRule::PerconaBackupMode);
        let rules = backup(Backup {
            insecure: true,
            ..DECLARED
        });
        assert_breaks!(rules, LintRule::PerconaBackupStorage { .. });
        let rules = backup(Backup {
            plain_http: true,
            ..DECLARED
        });
        assert_breaks!(rules, LintRule::PerconaBackupStorage { .. });
        let rules = backup(Backup {
            options: true,
            ..DECLARED
        });
        assert_breaks!(rules, LintRule::PerconaBackupOverride { .. });
    }

    /// The backup problems of a Percona cluster with `backup`: an unsafe storage, an unpinned
    /// backup image, or a problem of the backup agent.
    #[expect(
        clippy::disallowed_types,
        reason = "the backup spec is written as JSON"
    )]
    fn cluster_backup(backup: Option<serde_json::Value>) -> Vec<String> {
        let template = template(&[], &["percona-mongodb"]);
        let mut spec = json!({
            "image": format!("example.test/mongod{}", digest()),
            "initImage": format!("example.test/init{}", digest()),
        });
        if let Some(backup) = backup {
            spec["backup"] = backup;
        }
        let doc = json!({"spec": spec});
        let head = Head::default();
        let subject = subject(&template);
        let object = Object {
            subject: &subject,
            doc: &doc,
            head: &head,
            docs: &[],
            at: "mongodb",
        };
        let mut found = Found::default();
        super::percona(&object, &mut found);
        found
            .0
            .into_iter()
            .filter(|p| {
                matches!(&p.rule, LintRule::PerconaBackupStorage { .. })
                    || matches!(&p.rule, LintRule::PerconaDigest { field } if field == "backup.image")
                    || p.at.contains("/operand-")
            })
            .map(|p| p.to_string())
            .collect()
    }

    #[test]
    fn a_percona_cluster_checks_backups_only_when_enabled() {
        assert_eq!(
            cluster_backup(Some(json!({"enabled": false}))),
            Vec::<String>::new()
        );
        assert_eq!(cluster_backup(None), Vec::<String>::new());
        let unverified = json!({
            "enabled": true,
            "image": format!("example.test/pbm{}", digest()),
            "storages": {"s3": {"type": "s3", "s3": {
                "credentialsSecret": "backup-credentials",
                "endpointUrl": "https://s3.example.test",
                "insecureSkipTLSVerify": true,
            }}},
        });
        assert_ne!(cluster_backup(Some(unverified)), Vec::<String>::new());
        let unpinned = json!({"enabled": true, "image": "example.test/pbm:2", "storages": {}});
        assert_ne!(cluster_backup(Some(unpinned)), Vec::<String>::new());
    }
}
