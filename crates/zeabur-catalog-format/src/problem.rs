//! A problem the catalog's check finds: the file, the place in it and what is wrong.
//!
//! Each check names what it finds with its own enum. [`ProblemKind`] holds any of them, and only
//! their `Display` writes text.

use std::fmt;
use std::path::{Path, PathBuf};

use crate::check::StrictProblem;
use crate::common::{LicenseUrlProblem, PodSecurity, Profile};
use crate::order::OrderError;
use crate::template::Port;

/// One problem of one file.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Problem {
    /// The file, relative to the checkout, or `components` for the components together.
    pub file: PathBuf,
    /// Where in the file. Empty for the file as a whole.
    pub at: FieldPath,
    /// What is wrong.
    pub kind: ProblemKind,
}

impl fmt::Display for Problem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.at.is_empty() {
            write!(f, "{}: {}", self.file.display(), self.kind)
        } else {
            write!(f, "{}: {}: {}", self.file.display(), self.at, self.kind)
        }
    }
}

/// What a check finds.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ProblemKind {
    /// A field or value the types do not know, or text that is no valid file.
    Strict(StrictProblem),
    /// A `LicenseRef-` id and its link.
    LicenseUrl(LicenseUrlProblem),
    /// The file does not pass its schema in `schema/v1/`.
    Schema(SchemaProblem),
    /// A component, alone or with the others.
    Component(ComponentProblem),
    /// A template.
    Template(TemplateProblem),
    /// How the repository writes its files: licences, languages, READMEs, Kubernetes versions.
    Policy(PolicyProblem),
    /// A Helm value no comment explains.
    Values(ValuesProblem),
    /// The objects a component renders into.
    Render(RenderProblem),
    /// A new file of the API that readers of the published release cannot read.
    Compat(CompatProblem),
    /// The objects a template renders into with its e2e inputs.
    Lint(LintProblem),
    /// What an API server refuses of the rendered objects.
    DryRun(DryRunProblem),
}

impl fmt::Display for ProblemKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Strict(p) => p.fmt(f),
            Self::LicenseUrl(p) => p.fmt(f),
            Self::Schema(p) => p.fmt(f),
            Self::Component(p) => p.fmt(f),
            Self::Template(p) => p.fmt(f),
            Self::Policy(p) => p.fmt(f),
            Self::Values(p) => p.fmt(f),
            Self::Render(p) => p.fmt(f),
            Self::Compat(p) => p.fmt(f),
            Self::Lint(p) => p.fmt(f),
            Self::DryRun(p) => p.fmt(f),
        }
    }
}

impl From<StrictProblem> for ProblemKind {
    fn from(p: StrictProblem) -> Self {
        Self::Strict(p)
    }
}

impl From<LicenseUrlProblem> for ProblemKind {
    fn from(p: LicenseUrlProblem) -> Self {
        Self::LicenseUrl(p)
    }
}

impl From<SchemaProblem> for ProblemKind {
    fn from(p: SchemaProblem) -> Self {
        Self::Schema(p)
    }
}

impl From<ComponentProblem> for ProblemKind {
    fn from(p: ComponentProblem) -> Self {
        Self::Component(p)
    }
}

impl From<TemplateProblem> for ProblemKind {
    fn from(p: TemplateProblem) -> Self {
        Self::Template(p)
    }
}

impl From<PolicyProblem> for ProblemKind {
    fn from(p: PolicyProblem) -> Self {
        Self::Policy(p)
    }
}

impl From<ValuesProblem> for ProblemKind {
    fn from(p: ValuesProblem) -> Self {
        Self::Values(p)
    }
}

impl From<RenderProblem> for ProblemKind {
    fn from(p: RenderProblem) -> Self {
        Self::Render(p)
    }
}

impl From<DryRunProblem> for ProblemKind {
    fn from(p: DryRunProblem) -> Self {
        Self::DryRun(p)
    }
}

impl From<CompatProblem> for ProblemKind {
    fn from(p: CompatProblem) -> Self {
        Self::Compat(p)
    }
}

impl From<LintProblem> for ProblemKind {
    fn from(p: LintProblem) -> Self {
        Self::Lint(p)
    }
}

/// A rule that a template's rendered objects break, and where.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LintProblem {
    /// The object as `<kind>/<name>`, with `/<container>` or another part of it after, or the
    /// field of `template.yaml`, such as `security.unauthenticated`. Empty for the render as a
    /// whole.
    pub at: String,
    /// The rule.
    pub rule: LintRule,
}

impl fmt::Display for LintProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.at.is_empty() {
            self.rule.fmt(f)
        } else {
            write!(f, "{}: {}", self.at, self.rule)
        }
    }
}

/// A rule of the template lint.
///
/// A template renders only objects that stay in its namespace, run pinned images with bounded
/// resources at the Pod Security level of their service, and reach only the pods and networks
/// they declare.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LintRule {
    /// Two objects have the same API version, kind and name.
    DuplicateResource,
    /// An object names a namespace other than the template's.
    NamespaceEscape,
    /// A cluster-scoped object or RBAC.
    ClusterScoped,
    /// A kind that neither Kubernetes, cert-manager nor an operator rule covers.
    UnsupportedKind,
    /// A `storageClassName` or `storageClass` key anywhere in an object.
    StorageClass,
    /// A Secret: the installer creates Secrets from inputs.
    RenderedSecret,
    /// A core object or pod that does not read as its type.
    Invalid {
        /// What it should be, such as `Service` or `pod`.
        kind: String,
        /// Why it does not read.
        message: String,
    },
    /// A ServiceAccount that mounts its token.
    ServiceAccountToken,
    /// A `NodePort` or `LoadBalancer` Service, or one with external IPs.
    ExposedService,
    /// An Ingress of class `traefik`, and the template does not require `traefik`.
    TraefikProvider,
    /// An Ingress with a cert-manager `ClusterIssuer`, and the template does not require
    /// `cert-manager`.
    ClusterIssuerProvider,
    /// A pod without the template label set to the template ID.
    TemplateLabel,
    /// A pod breaks the Pod Security Standard of its service.
    PodSecurity {
        /// The level checked, or `None` when a security context does not read.
        level: Option<PodSecurity>,
        /// What breaks it, as Pod Security Admission says, or why the context does not read.
        refusal: String,
    },
    /// A pod mounts the service account token.
    PodTokenAutomount,
    /// A projected service account token volume.
    ProjectedToken,
    /// A sysctl other than `net.ipv4.ip_unprivileged_port_start` and `net.ipv4.ping_group_range`.
    UnsafeSysctl {
        /// The sysctl.
        name: String,
    },
    /// An image without a sha256 digest.
    ImageDigest,
    /// A `Bidirectional` volume mount.
    BidirectionalMount,
    /// A CPU or memory request or limit that is missing or not above zero.
    PositiveResource {
        /// Which, such as `requests.cpu`, or why the resources do not read.
        resource: String,
    },
    /// A long-running container without a readiness and a liveness probe.
    ProbesRequired,
    /// A NetworkPolicy selector or peer that selects pods other than the template's own or its
    /// clients.
    ForeignPods {
        /// `podSelector`, `ingress` or `egress`.
        selector: String,
    },
    /// A NetworkPolicy rule without a peer.
    UnboundedRule {
        /// `ingress` or `egress`.
        direction: String,
    },
    /// A NetworkPolicy peer without a selector or an IP block.
    EmptyPeer,
    /// An IP block of every address, and `security.risks` is empty.
    PublicNetworkWithoutRisk,
    /// `security.unauthenticated` names a service that no ingress NetworkPolicy selects.
    IngressPolicy {
        /// The service.
        service: String,
    },
    /// `security.unauthenticated` names a service the template does not declare.
    UnknownService {
        /// The service.
        service: String,
    },
    /// A field the lint has no rule for.
    UnsupportedOverride {
        /// The field.
        field: String,
    },
    /// An operator's pod gets a ServiceAccount that the template does not declare with its token
    /// disabled.
    OperandServiceAccount,
    /// An operator's pod gets sysctls.
    OperandSysctls,
    /// An operator's pod gets `seLinuxOptions`.
    OperandSeLinux,
    /// A CloudNativePG Cluster, and the template does not require `cloudnative-pg`.
    CnpgProvider,
    /// A CloudNativePG or Barman Cloud object, and the template does not require
    /// `cloudnative-pg`.
    CnpgResourceProvider,
    /// A CloudNativePG `imageName` without a digest.
    CnpgDigest,
    /// A CloudNativePG Cluster, and `security.risks` is empty.
    CnpgRisks,
    /// A CloudNativePG Cluster without a `RuntimeDefault` or `Localhost` seccomp profile.
    CnpgSeccomp,
    /// A CloudNativePG Cluster that runs PostgreSQL as user or group 0.
    CnpgRoot,
    /// A CloudNativePG plugin other than Barman Cloud.
    CnpgPlugin,
    /// A Percona MongoDB object, and the template does not require `percona-mongodb`.
    PerconaProvider,
    /// A Percona MongoDB cluster, and `security.risks` is empty.
    PerconaRisks,
    /// A Percona MongoDB image without a digest.
    PerconaDigest {
        /// The field, such as `initImage`.
        field: String,
    },
    /// A sharded Percona MongoDB cluster.
    PerconaSharding,
    /// A Percona MongoDB unsafe flag other than `replsetSize`.
    PerconaUnsafeFlag {
        /// The flag.
        flag: String,
    },
    /// A Percona MongoDB cluster without a replica set.
    PerconaNoReplicaSet,
    /// A Percona MongoDB volume other than a PersistentVolumeClaim.
    PerconaVolume,
    /// A Percona MongoDB backup, and the template does not require `percona-mongodb`.
    PerconaBackupProvider,
    /// A Percona MongoDB backup field the lint has no rule for.
    PerconaBackupOverride {
        /// The field.
        field: String,
    },
    /// A Percona MongoDB backup that is not logical with gzip.
    PerconaBackupMode,
    /// A Percona MongoDB backup without its cluster or storage.
    PerconaBackupTarget,
    /// A Percona MongoDB backup storage that is not S3 over verified HTTPS with credentials.
    PerconaBackupStorage {
        /// The storage, when the cluster names it.
        storage: Option<String>,
    },
    /// A Percona MongoDB backup of a cluster the template does not declare.
    PerconaBackupCluster,
    /// A Dragonfly, and the template does not require `dragonfly-operator`.
    DragonflyProvider,
    /// A Dragonfly, and `security.risks` is empty.
    DragonflyRisks,
    /// A Dragonfly image without a digest.
    DragonflyDigest,
    /// A Dragonfly without a password from a Secret.
    DragonflyPassword,
    /// A Dragonfly flag that opens a port or the admin console without a password.
    DragonflyFlag {
        /// The flag.
        flag: String,
    },
    /// A MariaDB Operator object, and the template does not require `mariadb-operator`.
    MariaDbProvider,
    /// A MariaDB, and `security.risks` is empty.
    MariaDbRisks,
    /// A MariaDB image without a digest.
    MariaDbDigest,
    /// A `mariaDbRef` to another namespace.
    MariaDbRefNamespace,
    /// A `mariaDbRef` to a MariaDB the template does not declare.
    MariaDbRefUndeclared,
    /// A MariaDB backup staging storage other than a PersistentVolumeClaim.
    MariaDbStagingStorage,
    /// A MariaDB backup that is not S3 with credentials and TLS.
    MariaDbBackupS3,
    /// A MariaDB User without a password from a Secret.
    MariaDbUserPassword,
    /// A MariaDB Grant on every database.
    GlobalGrant,
    /// A MariaDB Grant with `grantOption`.
    GrantOption,
    /// A MOCO `MySQLCluster`, and the template does not require `moco`.
    MocoProvider,
    /// A MOCO `MySQLCluster`, and `security.risks` is empty.
    MocoRisks,
    /// A Traefik Middleware, and the template does not require `traefik`.
    MiddlewareProvider,
    /// A Traefik Middleware other than one `ipAllowList`.
    IpAllowListOnly,
    /// An `ipAllowList` with keys other than `sourceRange`.
    IpAllowListKeys,
    /// An `ipAllowList` without a `sourceRange`.
    IpAllowListEmpty,
    /// A `sourceRange` entry that is no CIDR, or one of every address.
    IpAllowListCidr,
    /// A service of `template.yaml` with no workload or database object of its name.
    ServiceWithoutObject,
    /// The resources of a database object do not read.
    ServiceOperandResources {
        /// Why.
        message: String,
    },
    /// A quantity of a service, or of its objects, that is not one.
    ServiceQuantity {
        /// `requests` or `limits`.
        level: &'static str,
        /// `cpu` or `memory`.
        resource: &'static str,
        /// The text.
        value: String,
    },
    /// A request or limit of a service is not the sum of its containers.
    ServiceResources {
        /// `requests` or `limits`.
        level: &'static str,
        /// `cpu` or `memory`.
        resource: &'static str,
        /// What `template.yaml` gives.
        declared: String,
        /// The sum of the objects, or `None` when none gives it.
        rendered: Option<String>,
    },
    /// The ports of a service are not the named ports of its containers.
    ServicePorts {
        /// What `template.yaml` gives, by port number.
        declared: Vec<Port>,
        /// The named ports of the containers, by port number.
        named: Vec<Port>,
    },
    /// The template does not render with its e2e inputs.
    NotRendered {
        /// Why.
        message: String,
    },
    /// The e2e inputs do not read.
    E2eInputs {
        /// Why.
        message: String,
    },
    /// kubeconform refuses the objects for a Kubernetes version, or does not run.
    Kubeconform {
        /// The version.
        version: String,
        /// What kubeconform reports.
        message: String,
    },
}

impl fmt::Display for LintRule {
    #[expect(
        clippy::too_many_lines,
        reason = "one arm for each rule, with the text of the Go lint"
    )]
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let ports = |ports: &[Port]| {
            let items: Vec<String> = ports
                .iter()
                .map(|p| format!("{{{} {}}}", p.name, p.port))
                .collect();
            format!("[{}]", items.join(" "))
        };
        match self {
            Self::DuplicateResource => f.write_str("duplicate resource"),
            Self::NamespaceEscape => f.write_str("resource escapes target namespace"),
            Self::ClusterScoped => f.write_str("cluster-scoped resources and RBAC are forbidden"),
            Self::UnsupportedKind => f.write_str(
                "unsupported resource kind; the linter needs a namespaced safety adapter for it",
            ),
            Self::StorageClass => f.write_str(
                "storage class forbidden; volumes use the cluster's default StorageClass",
            ),
            Self::RenderedSecret => f.write_str(
                "rendered Secrets forbidden; use explicit inputs and preexisting Secrets",
            ),
            Self::Invalid { kind, message } => write!(f, "invalid {kind}: {message}"),
            Self::ServiceAccountToken => f.write_str("ServiceAccount must disable token automount"),
            Self::ExposedService => {
                f.write_str("externally exposed Service forbidden; use ingress")
            }
            Self::TraefikProvider => f.write_str("Traefik Ingress requires exact traefik provider"),
            Self::ClusterIssuerProvider => {
                f.write_str("ClusterIssuer annotation requires exact cert-manager provider")
            }
            Self::TemplateLabel => f.write_str(
                "pods must have the label templates.zeabur.com/template set to the template ID",
            ),
            Self::PodSecurity { level, refusal } => {
                f.write_str("pod breaks the Pod Security Standard of its service: ")?;
                if let Some(level) = level {
                    write!(f, "{}: ", level.as_str())?;
                }
                f.write_str(refusal)
            }
            Self::PodTokenAutomount => {
                f.write_str("automountServiceAccountToken must explicitly be false")
            }
            Self::ProjectedToken => f.write_str("projected service account tokens forbidden"),
            Self::UnsafeSysctl { name } => write!(f, "sysctl not in safe allowlist: {name}"),
            Self::ImageDigest => f.write_str("image must use immutable sha256 digest"),
            Self::BidirectionalMount => f.write_str("bidirectional mount propagation forbidden"),
            Self::PositiveResource { resource } => {
                write!(f, "positive resource required: {resource}")
            }
            Self::ProbesRequired => f.write_str("readiness and liveness probes required"),
            Self::ForeignPods { selector } => write!(
                f,
                "NetworkPolicy selectors must select the template's own pods, or in a peer its \
                 clients, by label: {selector}"
            ),
            Self::UnboundedRule { direction } => {
                write!(f, "unbounded rule forbidden: {direction}")
            }
            Self::EmptyPeer => f.write_str("empty peer permits all traffic"),
            Self::PublicNetworkWithoutRisk => {
                f.write_str("public network access requires risk documentation")
            }
            Self::IngressPolicy { service } => write!(
                f,
                "a service listed in security.unauthenticated needs an ingress NetworkPolicy \
                 whose podSelector names it: {service}"
            ),
            Self::UnknownService { service } => write!(
                f,
                "security.unauthenticated names a service the template does not declare: \
                 {service}"
            ),
            Self::UnsupportedOverride { field } => {
                write!(f, "unsupported override forbidden: {field}")
            }
            Self::OperandServiceAccount => {
                f.write_str("operand must reference declared token-disabled ServiceAccount")
            }
            Self::OperandSysctls => f.write_str("operand sysctl overrides forbidden"),
            Self::OperandSeLinux => {
                f.write_str("operand SELinux overrides require separate review")
            }
            Self::CnpgProvider => {
                f.write_str("CNPG workload requires exact cloudnative-pg provider")
            }
            Self::CnpgResourceProvider => {
                f.write_str("CNPG/Barman custom resources require cloudnative-pg")
            }
            Self::CnpgDigest => f.write_str("CNPG imageName must use sha256 digest"),
            Self::CnpgRisks => f.write_str(
                "operator-managed workload must document generated pod and service-account trust",
            ),
            Self::CnpgSeccomp => f.write_str("CNPG restricted seccomp required"),
            Self::CnpgRoot => f.write_str("CNPG root identity forbidden"),
            Self::CnpgPlugin => f.write_str("unsupported CNPG plugin forbidden"),
            Self::PerconaProvider => f.write_str("Percona operand requires exact percona-mongodb"),
            Self::PerconaRisks => f.write_str(
                "security.risks must describe the operator-generated pods, service account and \
                 probes",
            ),
            Self::PerconaDigest { field } => {
                write!(f, "Percona image must use sha256 digest: {field}")
            }
            Self::PerconaSharding => {
                f.write_str("sharded Percona topology requires separate safety adapter")
            }
            Self::PerconaUnsafeFlag { flag } => {
                write!(f, "unsafe Percona flag forbidden: {flag}")
            }
            Self::PerconaNoReplicaSet => f.write_str("at least one Percona replica set required"),
            Self::PerconaVolume => f.write_str("operand volumeSpec must use PVC only"),
            Self::PerconaBackupProvider => {
                f.write_str("Percona backup requires exact percona-mongodb")
            }
            Self::PerconaBackupOverride { field } => {
                write!(f, "unsupported backup override forbidden: {field}")
            }
            Self::PerconaBackupMode => {
                f.write_str("only gzip-compressed logical Percona backups supported")
            }
            Self::PerconaBackupTarget => {
                f.write_str("backup requires an explicit cluster and storage")
            }
            Self::PerconaBackupStorage { storage } => {
                f.write_str(
                    "backup must use the declared cluster's enabled S3 storage with explicit \
                     credentials and TLS verification",
                )?;
                storage
                    .as_ref()
                    .map_or(Ok(()), |storage| write!(f, ": {storage}"))
            }
            Self::PerconaBackupCluster => {
                f.write_str("backup cluster must be declared in this template")
            }
            Self::DragonflyProvider => f.write_str("Dragonfly requires exact dragonfly-operator"),
            Self::DragonflyRisks => f.write_str(
                "security.risks must describe the BUSL-1.1 licence and the operator-generated pods",
            ),
            Self::DragonflyDigest => f.write_str("Dragonfly image must use sha256 digest"),
            Self::DragonflyPassword => {
                f.write_str("Dragonfly authentication.passwordFromSecret required")
            }
            Self::DragonflyFlag { flag } => {
                write!(f, "unauthenticated Dragonfly flag forbidden: {flag}")
            }
            Self::MariaDbProvider => {
                f.write_str("MariaDB Operator resources require exact mariadb-operator")
            }
            Self::MariaDbRisks => f.write_str(
                "security.risks must describe the operator-generated pods and helper images",
            ),
            Self::MariaDbDigest => f.write_str("MariaDB image must use sha256 digest"),
            Self::MariaDbRefNamespace => f.write_str("mariaDbRef escapes target namespace"),
            Self::MariaDbRefUndeclared => {
                f.write_str("mariaDbRef must name a MariaDB declared in this template")
            }
            Self::MariaDbStagingStorage => f.write_str("backup stagingStorage must use PVC only"),
            Self::MariaDbBackupS3 => {
                f.write_str("backup must use S3 with explicit credentials and TLS verification")
            }
            Self::MariaDbUserPassword => f.write_str("User passwordSecretKeyRef required"),
            Self::GlobalGrant => {
                f.write_str("Grant must name one database; global privileges forbidden")
            }
            Self::GrantOption => f.write_str("Grant grantOption forbidden"),
            Self::MocoProvider => f.write_str("MySQLCluster requires exact moco"),
            Self::MocoRisks => f.write_str(
                "security.risks must describe the operator-generated pods and service account",
            ),
            Self::MiddlewareProvider => f.write_str("Middleware requires traefik"),
            Self::IpAllowListOnly => f.write_str("only explicit ipAllowList Middleware supported"),
            Self::IpAllowListKeys => f.write_str("ipAllowList may only contain sourceRange"),
            Self::IpAllowListEmpty => f.write_str("ipAllowList sourceRange required"),
            Self::IpAllowListCidr => {
                f.write_str("ipAllowList requires non-public, valid explicit CIDR")
            }
            Self::ServiceWithoutObject => f.write_str(
                "no DaemonSet, Deployment, StatefulSet or database object of the same name",
            ),
            Self::ServiceOperandResources { message } => write!(f, "resources: {message}"),
            Self::ServiceQuantity {
                level,
                resource,
                value,
            } => write!(
                f,
                "resources.{level}.{resource}: {value:?} is not a quantity"
            ),
            Self::ServiceResources {
                level,
                resource,
                declared,
                rendered,
            } => write!(
                f,
                "resources.{level}.{resource} is {declared}; the manifests give {}",
                rendered.as_deref().unwrap_or("<nil>")
            ),
            Self::ServicePorts { declared, named } => write!(
                f,
                "ports are {}; the containers' named ports are {}",
                ports(declared),
                ports(named)
            ),
            Self::NotRendered { message } => write!(f, "does not render: {message}"),
            Self::E2eInputs { message } => write!(f, "e2e inputs: {message}"),
            Self::Kubeconform { version, message } => write!(f, "kubeconform {version}: {message}"),
        }
    }
}

/// What the check of a new API against the published one finds.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CompatProblem {
    /// The file breaks a published schema, opened as its readers read it.
    PublishedSchema(SchemaProblem),
}

impl fmt::Display for CompatProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::PublishedSchema(p) => write!(
                f,
                "{p}: a change inside v1 must keep passing the published schemas; a breaking \
                 change goes to v2"
            ),
        }
    }
}

/// What an API server refuses of the rendered objects, applied with `--dry-run=server`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DryRunProblem {
    /// The API server refuses objects, or warns about a template's.
    Refused {
        /// What was sent, such as `single-node postgresql/cnpg` or `templates/ghost`.
        subject: String,
        /// What kubectl printed.
        output: String,
    },
    /// The API server refuses to update a release installed on one profile to the other.
    ProfileSwitch {
        /// The release.
        release: String,
        /// The profile it is installed on.
        from: Profile,
        /// The profile it switches to.
        to: Profile,
        /// What kubectl printed.
        output: String,
    },
    /// The API server refuses to update a release from the base's render on a profile.
    Upgrade {
        /// The release.
        release: String,
        /// The profile.
        profile: Profile,
        /// What kubectl printed.
        output: String,
    },
}

impl fmt::Display for DryRunProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Refused { subject, output } => {
                write!(f, "{subject}: the API server refuses it\n{}", output.trim())
            }
            Self::ProfileSwitch {
                release,
                from,
                to,
                output,
            } => write!(
                f,
                "the API server refuses to update release {release} from {} to {}. Give the \
                 field the same value in both profiles.\n{}",
                from.as_str(),
                to.as_str(),
                output.trim()
            ),
            Self::Upgrade {
                release,
                profile,
                output,
            } => write!(
                f,
                "the API server refuses to update release {release} from the base in {}. Change \
                 the component so the release updates in place, or add a `major` change file \
                 with the migration steps.\n{}",
                profile.as_str(),
                output.trim()
            ),
        }
    }
}

/// One rendering of a release: on a profile, for a Kubernetes version.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Rendering {
    /// The release.
    pub release: String,
    /// The profile.
    pub profile: Profile,
    /// The Kubernetes version.
    pub version: String,
}

impl fmt::Display for Rendering {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let profile = self.profile.as_str();
        write!(f, "{} {profile} {}", self.release, self.version)
    }
}

/// A rendered object by kind and name, such as `Deployment/web`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ObjectName {
    /// The kind.
    pub kind: String,
    /// The name.
    pub name: String,
}

impl fmt::Display for ObjectName {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}/{}", self.kind, self.name)
    }
}

/// What rendering a component and checking its objects finds.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RenderProblem {
    /// The release does not render.
    Failed {
        /// The rendering.
        on: Rendering,
        /// Why.
        message: String,
    },
    /// A CRD of the objects gives no schema for kubeconform.
    CrdSchemas {
        /// The rendering.
        on: Rendering,
        /// Why.
        message: String,
    },
    /// The rendered objects are no YAML stream.
    NotYaml {
        /// The rendering.
        on: Rendering,
        /// The parser's message.
        message: String,
    },
    /// A workload whose pod does not read as one.
    InvalidPod {
        /// The rendering.
        on: Rendering,
        /// The workload.
        object: ObjectName,
        /// Why.
        message: String,
    },
    /// A pod breaks the Pod Security Standard of its component.
    PodSecurity {
        /// The rendering.
        on: Rendering,
        /// The workload.
        object: ObjectName,
        /// The level of the component.
        level: PodSecurity,
        /// What breaks it, as Pod Security Admission says.
        refusal: String,
    },
    /// An image without a digest that the release does not allow.
    UnpinnedImage {
        /// The rendering.
        on: Rendering,
        /// The workload.
        object: ObjectName,
        /// The container.
        container: String,
        /// The image.
        image: String,
    },
    /// Two renders of the same values give different objects: the chart generates a value.
    NotReproducible {
        /// The rendering.
        on: Rendering,
    },
    /// The component serves an admission webhook but has no health check, so what needs it can
    /// install before it answers.
    WebhookWithoutHealth,
    /// kubeconform refuses objects rendered for a Kubernetes version, or does not run.
    Kubeconform {
        /// What kubeconform reports.
        message: String,
    },
}

impl fmt::Display for RenderProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Failed { on, message }
            | Self::CrdSchemas { on, message }
            | Self::NotYaml { on, message } => write!(f, "{on}: {message}"),
            Self::InvalidPod {
                on,
                object,
                message,
            } => write!(f, "{on}: {object}: invalid pod: {message}"),
            Self::PodSecurity {
                on,
                object,
                level,
                refusal,
            } => write!(
                f,
                "{on}: {object}: pod breaks the Pod Security Standard of its service: {}: \
                 {refusal}",
                level.as_str()
            ),
            Self::UnpinnedImage {
                on,
                object,
                container,
                image,
            } => write!(
                f,
                "{on}: {object} {container}: {image} needs a sha256 digest \
                 (<tag>@sha256:<digest>)"
            ),
            Self::NotReproducible { on } => write!(
                f,
                "{on}: output differs between renders; supply generated values as inputs"
            ),
            Self::WebhookWithoutHealth => f.write_str(
                "serves an admission webhook but declares no health, so what needs it can \
                 install before it answers",
            ),
            Self::Kubeconform { message } => write!(f, "kubeconform: {message}"),
        }
    }
}

/// What the check of a component's Helm values finds: every value says why it is set.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ValuesProblem {
    /// No comment explains the value at this path inside the values.
    Unexplained {
        /// The path of the value inside the values, such as `controller.replicas`.
        value: FieldPath,
    },
    /// The values are no YAML.
    NotYaml {
        /// The parser's message.
        message: String,
    },
}

impl fmt::Display for ValuesProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Unexplained { value } => write!(f, "{value} has no comment saying why"),
            Self::NotYaml { message } => write!(f, "invalid YAML: {message}"),
        }
    }
}

/// What the repository's policy finds: the files install without it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PolicyProblem {
    /// A licence is not OSI-approved.
    NotOsiApproved {
        /// The SPDX id.
        license: String,
    },
    /// A licence is opt-in only, and the component is recommended on a profile.
    OptInRecommended {
        /// The SPDX id.
        license: String,
        /// The profile.
        profile: Profile,
    },
    /// A `locales` map lacks a language that `config/locales.yaml` requires.
    MissingLocale {
        /// The language.
        lang: String,
    },
    /// The policy has no README headings for a language that `config/locales.yaml` requires.
    NoHeadings {
        /// The language.
        lang: String,
    },
    /// The README of a required language is missing.
    MissingReadme,
    /// A README has a level-1 heading.
    Level1Heading {
        /// The line.
        line: String,
        /// The first level-2 heading.
        first: String,
    },
    /// A level-2 heading is not one of the README's, or out of order.
    Heading {
        /// The line.
        line: String,
        /// The level-2 headings, in order.
        headings: Vec<String>,
    },
    /// A README lacks a section it needs.
    MissingSection {
        /// The heading.
        heading: String,
    },
    /// A template README does not say where to read a generated input.
    SecretNotNamed {
        /// The section that should name it.
        section: String,
        /// The Secret.
        secret: String,
        /// The input.
        input: String,
    },
    /// `kubeVersion` leaves out a version that `config/kubernetes.yaml` tests.
    LeavesOut {
        /// The range.
        range: String,
        /// The version.
        version: String,
    },
    /// `config/kubernetes.yaml` names something that is no version.
    NotAVersion {
        /// The text.
        version: String,
    },
    /// `kubeVersion` is no range the check reads.
    NotARange {
        /// The text.
        range: String,
    },
}

impl fmt::Display for PolicyProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::NotOsiApproved { license } => write!(f, "{license} is not OSI-approved"),
            Self::OptInRecommended { license, profile } => write!(
                f,
                "{license} is opt-in only, so {} cannot be recommended",
                profile.as_str()
            ),
            Self::MissingLocale { lang } => write!(f, "no {lang}"),
            Self::NoHeadings { lang } => write!(f, "no README headings for {lang}"),
            Self::MissingReadme => f.write_str("missing"),
            Self::Level1Heading { line, first } => {
                write!(f, "level-1 heading {line:?}; start with ## {first}")
            }
            Self::Heading { line, headings } => write!(
                f,
                "heading {line:?}; the level-2 headings are {}, in that order",
                headings.join(", ")
            ),
            Self::MissingSection { heading } => write!(f, "no ## {heading}"),
            Self::SecretNotNamed {
                section,
                secret,
                input,
            } => write!(
                f,
                "{section} does not name Secret {secret} of input {input}"
            ),
            Self::LeavesOut { range, version } => write!(
                f,
                "{range} leaves out {version}, which config/kubernetes.yaml tests"
            ),
            Self::NotAVersion { version } => write!(f, "{version:?} is not a version"),
            Self::NotARange { range } => write!(f, "{range:?} is not a range it reads"),
        }
    }
}

/// What the check finds in a template.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TemplateProblem {
    /// A client cannot render it: its manifests do not parse, or two inputs share a name or a
    /// Secret key.
    Broken {
        /// The renderer's message.
        message: String,
    },
    /// The directory name is no DNS label of at most 49 characters, so the e2e namespace
    /// `templates-e2e-<id>` is none either.
    Id,
    /// A component has the same ID.
    SameIdAsComponent,
    /// Two services have this name.
    DuplicateService {
        /// The name.
        name: String,
    },
    /// `metadata.version` has no `# renovate: depName=<image>` comment on the line above it.
    VersionWithoutImage,
    /// `metadata.version` is not the tag, pinned with a digest, of the image it names in the
    /// manifests.
    VersionNotPinned {
        /// The image.
        image: String,
    },
    /// What the template requires has no install order on a profile.
    Requires {
        /// The profile.
        profile: Profile,
        /// Why.
        error: OrderError,
    },
}

impl fmt::Display for TemplateProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Broken { message } => f.write_str(message),
            Self::Id => {
                f.write_str("the directory name must be a DNS label of at most 49 characters")
            }
            Self::SameIdAsComponent => f.write_str("a component has the same ID"),
            Self::DuplicateService { name } => write!(f, "duplicate service {name}"),
            Self::VersionWithoutImage => {
                f.write_str(r##"needs "# renovate: depName=<image>" on the line above it"##)
            }
            Self::VersionNotPinned { image } => write!(
                f,
                "must be the tag of {image} in manifests.yaml, pinned with a digest"
            ),
            Self::Requires { profile, error } => write!(f, "on {}: {error}", profile.as_str()),
        }
    }
}

/// A rule of a schema that a value breaks.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SchemaProblem {
    /// The validator's message.
    pub message: String,
}

impl fmt::Display for SchemaProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

/// What the check finds in a component, alone or with the others.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ComponentProblem {
    /// The directory name is no id.
    Id,
    /// `provides.capability` is the id of another component, which an installer reads first.
    CapabilityIsComponent {
        /// The capability.
        capability: String,
    },
    /// `urlDigests` has not one digest for each URL.
    UrlDigests {
        /// How many URLs the release has.
        urls: usize,
    },
    /// An input has a path and the component no chart release for it.
    InputWithoutRelease,
    /// A required input has no value CI can install it with.
    InputWithoutValue,
    /// `provides.registry.ca` is in a namespace the component does not own.
    CaNamespace {
        /// The namespace.
        namespace: String,
    },
    /// An internal CRD is not in `detection.crds`.
    InternalCrd {
        /// The CRD.
        crd: String,
    },
    /// An input generated as a cluster IP has no path.
    ClusterIpWithoutPath,
    /// `networkPolicies` is no list of objects.
    NetworkPolicies {
        /// The parser's message.
        message: String,
    },
    /// An object of `networkPolicies` is no `NetworkPolicy` in a namespace the component owns.
    NetworkPolicy {
        /// Its kind.
        kind: String,
        /// Its name.
        name: String,
    },
    /// Two works have the same name.
    WorkTwice {
        /// The name.
        name: String,
    },
    /// A quantity of a work.
    Quantity(QuantityProblem),
    /// The component contributes values and has no Helm release, by which an installer finds it.
    ContributorWithoutHelm,
    /// A contribution names no component.
    UnknownTarget {
        /// The id it names.
        component: String,
    },
    /// A contribution names a release its component does not have.
    UnknownTargetRelease {
        /// The component.
        component: String,
        /// The release it names.
        release: String,
    },
    /// The components have no install order on a profile.
    InstallOrder {
        /// The profile.
        profile: Profile,
        /// Why.
        error: OrderError,
    },
}

impl fmt::Display for ComponentProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Id => f.write_str(
                "the directory name must be lowercase letters, digits and single hyphens",
            ),
            Self::CapabilityIsComponent { capability } => {
                write!(f, "{capability} is the ID of another component")
            }
            Self::UrlDigests { urls } => {
                write!(f, "needs one digest for each of the {urls} urls")
            }
            Self::InputWithoutRelease => f.write_str("no chart release for its path"),
            Self::InputWithoutValue => {
                f.write_str("required, so set default, testValue or generate for CI to install it")
            }
            Self::CaNamespace { namespace } => write!(f, "{namespace} is none of its namespaces"),
            Self::InternalCrd { crd } => write!(f, "{crd} is not in detection.crds"),
            Self::ClusterIpWithoutPath => f.write_str("generate clusterIP needs a path"),
            Self::NetworkPolicies { message } => f.write_str(message),
            Self::NetworkPolicy { kind, name } => write!(
                f,
                "{kind} {name} must be a NetworkPolicy in one of its namespaces"
            ),
            Self::WorkTwice { name } => write!(f, "{name} is listed twice"),
            Self::Quantity(p) => p.fmt(f),
            Self::ContributorWithoutHelm => f.write_str("needs a helm release of its own"),
            Self::UnknownTarget { component } => write!(f, "unknown target {component}"),
            Self::UnknownTargetRelease { component, release } => {
                write!(f, "{component} has no release {release}")
            }
            Self::InstallOrder { profile, error } => write!(f, "{}: {error}", profile.as_str()),
        }
    }
}

/// A quantity that is not one, or not above zero.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum QuantityProblem {
    /// It is no Kubernetes quantity.
    NotAQuantity {
        /// The text.
        value: String,
    },
    /// It is zero or less.
    NotAboveZero {
        /// The text.
        value: String,
    },
}

impl fmt::Display for QuantityProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::NotAQuantity { value } => write!(f, "{value} is not a quantity"),
            Self::NotAboveZero { value } => write!(f, "{value} is not above zero"),
        }
    }
}

/// One step from a value into what it holds.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Segment {
    /// A field of an object.
    Key(String),
    /// An item of a list.
    Index(usize),
}

/// The way from the top of a file to a value, written as `health[0].condition`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct FieldPath(Vec<Segment>);

impl FieldPath {
    /// The top of the file.
    #[must_use]
    pub const fn root() -> Self {
        Self(Vec::new())
    }

    /// The field `key` of the value here.
    #[must_use]
    pub fn key(mut self, key: impl Into<String>) -> Self {
        self.0.push(Segment::Key(key.into()));
        self
    }

    /// The item `index` of the list here.
    #[must_use]
    pub fn index(mut self, index: usize) -> Self {
        self.0.push(Segment::Index(index));
        self
    }

    /// This path, then `rest`.
    #[must_use]
    pub fn join(mut self, rest: Self) -> Self {
        self.0.extend(rest.0);
        self
    }

    /// Whether this is the top of the file.
    #[must_use]
    pub const fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
}

impl fmt::Display for FieldPath {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for (i, segment) in self.0.iter().enumerate() {
            match segment {
                Segment::Key(key) if i == 0 => write!(f, "{key}")?,
                Segment::Key(key) => write!(f, ".{key}")?,
                Segment::Index(index) => write!(f, "[{index}]")?,
            }
        }
        Ok(())
    }
}

/// The problems of one file, as a check finds them.
#[derive(Debug)]
pub struct Problems {
    file: PathBuf,
    found: Vec<Problem>,
}

impl Problems {
    /// No problems yet in `file`.
    #[must_use]
    pub fn new(file: &Path) -> Self {
        Self {
            file: file.to_owned(),
            found: Vec::new(),
        }
    }

    /// Adds `kind` at `at`.
    pub fn push(&mut self, at: FieldPath, kind: impl Into<ProblemKind>) {
        self.found.push(Problem {
            file: self.file.clone(),
            at,
            kind: kind.into(),
        });
    }

    /// Whether none were found.
    #[must_use]
    pub const fn is_empty(&self) -> bool {
        self.found.is_empty()
    }

    /// The problems found, in order.
    #[must_use]
    pub fn into_vec(self) -> Vec<Problem> {
        self.found
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::{FieldPath, Problem};
    use crate::check::StrictProblem;

    #[test]
    fn a_path_reads_as_its_keys_and_indexes() {
        let at = FieldPath::root().key("health").index(0).key("condition");
        assert_eq!(at.to_string(), "health[0].condition");
        assert_eq!(FieldPath::root().index(1).key("a").to_string(), "[1].a");
    }

    #[test]
    fn a_problem_names_its_file_and_place() {
        let problem = Problem {
            file: PathBuf::from("components/web/component.yaml"),
            at: FieldPath::root().key("distributions").index(1),
            kind: StrictProblem::UnknownValue.into(),
        };
        assert_eq!(
            problem.to_string(),
            "components/web/component.yaml: distributions[1]: unknown value"
        );
    }
}
