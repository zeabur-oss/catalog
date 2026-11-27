//! `components/<id>/component.yaml` [component-format].
//!
//! [component-format]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/component-format.mdx

use std::collections::BTreeMap;

#[cfg(feature = "schema")]
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use crate::common::{
    ApiVersion, Distribution, Generate, LicenseUrls, Locales, Mode, PerProfile, PodSecurity,
    Profile, Reference, SecretRef, Summary, WorkloadKind,
};

/// The `kind` of a component file.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
pub enum ComponentKind {
    /// `Component`.
    Component,
}

/// One upstream project that a cluster administrator installs.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Component {
    /// `catalog.zeabur.com/v1`.
    pub api_version: ApiVersion,
    /// `Component`.
    pub kind: ComponentKind,
    /// Its name and summary.
    pub metadata: Metadata,
    /// Whether each profile installs it by default.
    pub defaults: PerProfile<Mode>,
    /// The distributions it runs on and e2e tests it on.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    #[cfg_attr(feature = "schema", schemars(extend("uniqueItems" = true)))]
    pub distributions: Vec<Distribution>,
    /// The Kubernetes versions it supports, as a `SemVer` range like Helm's `kubeVersion`.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub kube_version: String,
    /// It keeps state in volumes, so uninstalling it loses data.
    #[serde(default)]
    pub persists_data: bool,
    /// The components installed before it.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("uniqueItems" = true)))]
    pub requires: Vec<Reference>,
    /// Values an installer asks for or generates.
    #[serde(default)]
    pub inputs: Vec<Input>,
    /// What it installs, in order.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub releases: Vec<Release>,
    /// What it offers other software in the cluster.
    #[serde(default)]
    pub provides: Option<Provides>,
    /// The CRDs of `detection.crds` whose objects it writes itself, so they do not mean anyone
    /// uses it.
    #[serde(default, rename = "internalCRDs")]
    pub internal_crds: Vec<String>,
    /// What it needs beyond its measured idle use.
    #[serde(default)]
    pub resources: Option<Resources>,
    /// `NetworkPolicy` objects for its namespaces, applied before the releases.
    #[serde(default)]
    pub network_policies: Option<String>,
    /// Values it adds to releases of other components.
    #[serde(default)]
    pub contributes: Vec<Contribution>,
    /// How to recognise an existing installation.
    pub detection: Detection,
    /// What its RBAC lets it do and what its namespaces must allow.
    pub security: Security,
    /// Read-only checks of an installed component.
    #[serde(default)]
    pub health: Vec<Health>,
    /// Extra e2e checks after install.
    #[serde(default)]
    pub smoke: Vec<Smoke>,
    /// What the upstream keeps on uninstall.
    #[serde(default)]
    pub leftovers: Vec<String>,
}

impl AsRef<Self> for Component {
    fn as_ref(&self) -> &Self {
        self
    }
}

impl Component {
    /// The capability it provides.
    #[must_use]
    pub fn capability(&self) -> Option<&str> {
        self.provides.as_ref()?.capability.as_deref()
    }

    /// The release named `name`.
    #[must_use]
    pub fn release(&self, name: &str) -> Option<&Release> {
        self.releases.iter().find(|r| r.name() == Some(name))
    }

    /// The release whose values take a `path` input: the input's `release`, else the first chart
    /// release.
    #[must_use]
    pub fn input_release(&self, input: &Input) -> Option<&Release> {
        input.release.as_deref().map_or_else(
            || self.releases.iter().find(|r| matches!(r, Release::Helm(_))),
            |name| self.release(name),
        )
    }

    /// Its mode on `profile`.
    #[must_use]
    pub const fn mode(&self, profile: Profile) -> Mode {
        self.defaults.mode(profile)
    }

    /// Whether this reader can install it: every release type, its pod security level and the
    /// `generate` type of every required input without a default are known. One it cannot
    /// install is not offered.
    #[must_use]
    pub fn known(&self) -> bool {
        !self.releases.iter().any(|r| matches!(r, Release::Unknown))
            && self.security.pod_security != PodSecurity::Unknown
            && !self
                .inputs
                .iter()
                .any(|i| i.required && i.default.is_none() && i.generate == Some(Generate::Unknown))
    }
}

/// A component's name and summary.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Metadata {
    /// The name for people.
    pub name: String,
    /// What it gives the cluster, in one plain sentence.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::LINE)))]
    pub summary: String,
    /// The summary in other languages.
    #[cfg_attr(feature = "schema", schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE})))]
    pub locales: Locales<Summary>,
    /// An SPDX expression.
    pub license: String,
    /// The URL of the text of each `LicenseRef-` licence in `license`, by its id.
    #[serde(default, skip_serializing_if = "LicenseUrls::is_empty")]
    #[cfg_attr(feature = "schema", schemars(extend(
        "propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LICENSE_REF},
        "additionalProperties" = {"type": "string", "format": "uri", "pattern": crate::schema::pattern::HTTPS}
    )))]
    pub license_urls: LicenseUrls,
    /// The project's home page.
    #[cfg_attr(feature = "schema", schemars(url))]
    pub homepage: String,
}

/// What a component's RBAC lets it do and what its namespaces must allow.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Security {
    /// What its RBAC lets it do.
    pub summary: String,
    /// The level its namespaces need.
    pub pod_security: PodSecurity,
}

/// What a component offers other software in the cluster.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("minProperties" = 1)))]
pub struct Provides {
    /// The capability it offers, such as `logs`.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::NAME)))]
    pub capability: Option<String>,
    /// An OCI registry in the cluster that nodes pull from.
    #[serde(default)]
    pub registry: Option<Registry>,
}

/// An OCI registry in the cluster that nodes pull from.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Registry {
    /// The `host:port` that images name.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::HOST_PORT)))]
    pub host: String,
    /// The CA that signs its serving certificate.
    pub ca: SecretRef,
}

/// What a component needs beyond its measured idle use.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Resources {
    /// Each kind of work it does, such as a build.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub work: Vec<Work>,
}

/// One kind of work a component does.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("anyOf" = [{"required": ["requests"]}, {"required": ["limits"]}, {"required": ["use"]}])))]
pub struct Work {
    /// Its name.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::NAME)))]
    pub name: String,
    /// The requests of each pod doing one unit of work.
    #[serde(default)]
    pub requests: Option<Quantities>,
    /// The limits of each pod doing one unit of work.
    #[serde(default)]
    pub limits: Option<Quantities>,
    /// What one unit of work uses on top of the idle use.
    #[serde(default, rename = "use")]
    pub uses: Option<Quantities>,
}

/// Kubernetes quantities by resource name.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("minProperties" = 1)))]
pub struct Quantities {
    /// CPU, such as `100m`.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub cpu: Option<String>,
    /// Memory, such as `256Mi`.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub memory: Option<String>,
}

/// One install step of a component, by its `type`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(tag = "type", rename_all = "camelCase")]
pub enum Release {
    /// A Helm chart.
    Helm(HelmRelease),
    /// Files downloaded from URLs.
    Manifest(ManifestRelease),
    /// Objects written in the file.
    Inline(InlineRelease),
    /// A type this reader does not know. A component with it is not offered.
    #[serde(other)]
    #[cfg_attr(feature = "schema", schemars(skip))]
    Unknown,
}

impl Release {
    /// Its name in the file.
    #[must_use]
    pub fn name(&self) -> Option<&str> {
        match self {
            Self::Helm(r) => Some(&r.name),
            Self::Manifest(r) => Some(&r.name),
            Self::Inline(r) => Some(&r.name),
            Self::Unknown => None,
        }
    }

    /// The namespace it installs into.
    #[must_use]
    pub fn namespace(&self) -> Option<&str> {
        match self {
            Self::Helm(r) => Some(&r.namespace),
            Self::Manifest(r) => Some(&r.namespace),
            Self::Inline(r) => Some(&r.namespace),
            Self::Unknown => None,
        }
    }

    /// The CRDs whose presence skips it.
    #[must_use]
    pub fn skip_if_present(&self) -> &[String] {
        match self {
            Self::Helm(r) => &r.skip_if_present,
            Self::Manifest(r) => &r.skip_if_present,
            Self::Inline(r) => &r.skip_if_present,
            Self::Unknown => &[],
        }
    }

    /// The chart release, if it is one.
    #[must_use]
    pub const fn helm(&self) -> Option<&HelmRelease> {
        match self {
            Self::Helm(r) => Some(r),
            Self::Manifest(_) | Self::Inline(_) | Self::Unknown => None,
        }
    }
}

/// A release of a Helm chart.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct HelmRelease {
    /// Its name in the file.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::NAME)))]
    pub name: String,
    /// The namespace it installs into.
    pub namespace: String,
    /// The Helm release name, when it is not `name`.
    #[serde(default)]
    pub release_name: Option<String>,
    /// Skip it when these CRDs exist.
    #[serde(default)]
    pub skip_if_present: Vec<String>,
    /// `oci://registry/path/chart`, or a chart name in `repo`.
    pub chart: String,
    /// The HTTP chart repository.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::HTTPS)))]
    pub repo: Option<String>,
    /// The chart version.
    pub version: String,
    /// The OCI manifest digest, or the SHA-256 of the chart archive.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::DIGEST)))]
    pub chart_digest: String,
    /// Images, without tag, that the chart cannot pin by digest.
    #[serde(default, rename = "unpinnedImages")]
    pub unpinned_images: Vec<String>,
    /// Values for every profile, as YAML.
    #[serde(default)]
    pub values: Option<String>,
    /// Values merged over `values` on each profile, as YAML.
    #[serde(default)]
    pub profiles: ProfileValues,
}

impl HelmRelease {
    /// The Helm release name.
    #[must_use]
    pub fn helm_name(&self) -> &str {
        self.release_name.as_deref().unwrap_or(&self.name)
    }
}

/// Values for some profiles, as YAML.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "kebab-case")]
pub struct ProfileValues {
    /// On `single-node`.
    #[serde(default)]
    pub single_node: Option<String>,
    /// On `multi-node`.
    #[serde(default)]
    pub multi_node: Option<String>,
}

impl ProfileValues {
    /// The values on `profile`.
    #[must_use]
    pub fn get(&self, profile: Profile) -> Option<&str> {
        match profile {
            Profile::SingleNode => self.single_node.as_deref(),
            Profile::MultiNode => self.multi_node.as_deref(),
        }
    }
}

/// A release of files downloaded from URLs.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct ManifestRelease {
    /// Its name in the file.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::NAME)))]
    pub name: String,
    /// The namespace it installs into.
    pub namespace: String,
    /// Skip it when these CRDs exist.
    #[serde(default)]
    pub skip_if_present: Vec<String>,
    /// The URLs, with `{version}` for `version`.
    #[cfg_attr(feature = "schema", schemars(inner(regex(pattern = crate::schema::pattern::VERSION_FIELD))))]
    pub urls: Vec<String>,
    /// The version.
    pub version: String,
    /// The `sha256:<hex>` digest of each file, in the order of `urls`.
    #[cfg_attr(feature = "schema", schemars(inner(regex(pattern = crate::schema::pattern::DIGEST))))]
    pub url_digests: Vec<String>,
}

impl ManifestRelease {
    /// The URLs with `{version}` replaced.
    #[must_use]
    pub fn resolved_urls(&self) -> Vec<String> {
        self.urls
            .iter()
            .map(|u| u.replace("{version}", &self.version))
            .collect()
    }
}

/// A release of objects written in the file.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct InlineRelease {
    /// Its name in the file.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::NAME)))]
    pub name: String,
    /// The namespace it installs into.
    pub namespace: String,
    /// Skip it when these CRDs exist.
    #[serde(default)]
    pub skip_if_present: Vec<String>,
    /// The objects, as YAML.
    pub manifests: String,
}

/// A value an installer asks for or generates.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("oneOf" = [{"required": ["path"]}, {"required": ["secret"]}])))]
pub struct Input {
    /// Its name, camelCase.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::INPUT)))]
    pub name: String,
    /// What it is, in English.
    pub description: String,
    /// The release whose values take a `path` input; the first chart release when absent.
    #[serde(default)]
    pub release: Option<String>,
    /// Where it goes in the release's values: a dotted path, where a number indexes a list.
    #[serde(default)]
    pub path: Option<String>,
    /// The Secret key it is stored in, created before install.
    #[serde(default)]
    pub secret: Option<SecretRef>,
    /// How an installer makes it when it is not given.
    #[serde(default)]
    pub generate: Option<Generate>,
    /// An install needs a value.
    #[serde(default)]
    pub required: bool,
    /// The value when none is given.
    #[serde(default)]
    pub default: Option<InputValue>,
    /// The value CI installs with.
    #[serde(default)]
    pub test_value: Option<InputValue>,
}

/// The value of an input.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(untagged)]
pub enum InputValue {
    /// `true` or `false`.
    Flag(bool),
    /// A whole number.
    Integer(i64),
    /// Text.
    Text(String),
    /// Values by key, set at a `path` as one object, such as a chart value that is a map.
    Map(BTreeMap<String, Self>),
}

impl InputValue {
    /// The value in a chart's values.
    #[must_use]
    #[expect(
        clippy::disallowed_types,
        reason = "a value in Helm values, which are YAML of any shape"
    )]
    pub fn to_json(&self) -> serde_json::Value {
        match self {
            Self::Flag(b) => (*b).into(),
            Self::Integer(i) => (*i).into(),
            Self::Text(s) => s.clone().into(),
            Self::Map(m) => m
                .iter()
                .map(|(k, v)| (k.clone(), v.to_json()))
                .collect::<serde_json::Map<_, _>>()
                .into(),
        }
    }

    /// The value as a Secret stores it: a map as JSON. `check` refuses a map in a Secret input.
    #[must_use]
    pub fn text(&self) -> String {
        match self {
            Self::Flag(b) => b.to_string(),
            Self::Integer(i) => i.to_string(),
            Self::Text(s) => s.clone(),
            Self::Map(_) => self.to_json().to_string(),
        }
    }
}

/// Values a component adds to a release of another component.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Contribution {
    /// The release that receives them.
    pub target: ReleaseRef,
    /// Helm values, as YAML.
    pub values: String,
}

/// A release of a component.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct ReleaseRef {
    /// The component's id.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::ID)))]
    pub component: String,
    /// The release's name in its file.
    pub release: String,
}

/// How to recognise an existing installation. Any one match means it is there.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("minProperties" = 1)))]
pub struct Detection {
    /// `CustomResourceDefinition` names.
    #[serde(default)]
    pub crds: Vec<String>,
    /// `APIService` names.
    #[serde(default)]
    pub api_services: Vec<String>,
    /// `GatewayClass` controller names.
    #[serde(default)]
    pub gateway_controllers: Vec<String>,
    /// `IngressClass` controller names.
    #[serde(default)]
    pub ingress_controllers: Vec<String>,
    /// CSI drivers of the `PersistentVolumes` it provides.
    #[serde(default)]
    pub csi_drivers: Vec<String>,
    /// Helm chart names.
    #[serde(default)]
    pub helm_charts: Vec<String>,
    /// Add-ons of a distribution that install the same upstream.
    #[serde(default)]
    pub addons: Vec<Addon>,
    /// Workloads whose labels show the upstream.
    #[serde(default)]
    pub workloads: Vec<DetectedWorkload>,
}

impl Detection {
    /// The names of the add-ons that `distribution` packages.
    pub fn addons_on(&self, distribution: Distribution) -> impl Iterator<Item = &str> {
        self.addons
            .iter()
            .filter(move |a| a.distribution == distribution)
            .map(|a| a.name.as_str())
    }
}

/// An add-on of a distribution.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Addon {
    /// The distribution.
    pub distribution: Distribution,
    /// The name of its `HelmChart` in `kube-system`.
    pub name: String,
}

/// A workload whose labels show the upstream.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct DetectedWorkload {
    /// Its kind.
    pub kind: WorkloadKind,
    /// Labels that must all be there with these values.
    pub labels: BTreeMap<String, String>,
}

/// A read-only check of an installed component: one `kubectl wait` or `kubectl rollout status`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("if" = {"properties": {"resource": {"properties": {"kind": {"enum": ["DaemonSet", "Deployment", "StatefulSet"]}}}}}, "then" = {"not": {"required": ["condition"]}, "properties": {"resource": {"not": {"required": ["all"]}}}}, "else" = {"required": ["condition"]})))]
pub struct Health {
    /// Its name.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::NAME)))]
    pub name: String,
    /// The objects to wait for.
    pub resource: HealthResource,
    /// The condition the objects reach. A workload kind has none: it is ready when its rollout
    /// is complete.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::CONDITION_NAME)))]
    pub condition: Option<String>,
    /// How long to wait, such as `60s`.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::TIMEOUT)))]
    pub timeout: String,
    /// What to do when the check fails, in English.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub advice: String,
}

/// The objects of one kind a health check waits for: by `name`, by `selector`, or `all`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("oneOf" = [{"required": ["name"]}, {"required": ["selector"]}, {"required": ["all"]}], "if" = {"properties": {"kind": {"enum": ["DaemonSet", "Deployment", "Node.longhorn.io", "Pod", "StatefulSet"]}}}, "then" = {"required": ["namespace"]}, "else" = {"not": {"required": ["namespace"]}})))]
pub struct HealthResource {
    /// Their kind.
    pub kind: HealthKind,
    /// The name of one object.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub name: Option<String>,
    /// The labels of the objects.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("minProperties" = 1)))]
    pub selector: Option<BTreeMap<String, String>>,
    /// Every object of the kind.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("const" = true)))]
    pub all: bool,
    /// Their namespace, for a namespaced kind.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub namespace: Option<String>,
}

/// A kind that a health check can wait for.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
pub enum HealthKind {
    /// `APIService`.
    #[serde(rename = "APIService")]
    ApiService,
    /// `DaemonSet`.
    DaemonSet,
    /// `Deployment`.
    Deployment,
    /// `GatewayClass`.
    GatewayClass,
    /// Longhorn's `Node`.
    #[serde(rename = "Node.longhorn.io")]
    LonghornNode,
    /// `Pod`.
    Pod,
    /// `StatefulSet`.
    StatefulSet,
}

impl HealthKind {
    /// The resource that kubectl names, as `plural.group`.
    #[must_use]
    pub const fn resource(self) -> &'static str {
        match self {
            Self::ApiService => "apiservices.apiregistration.k8s.io",
            Self::DaemonSet => "daemonsets.apps",
            Self::Deployment => "deployments.apps",
            Self::GatewayClass => "gatewayclasses.gateway.networking.k8s.io",
            Self::LonghornNode => "nodes.longhorn.io",
            Self::Pod => "pods",
            Self::StatefulSet => "statefulsets.apps",
        }
    }

    /// Whether its objects live in a namespace.
    #[must_use]
    pub const fn namespaced(self) -> bool {
        match self {
            Self::ApiService | Self::GatewayClass => false,
            Self::DaemonSet
            | Self::Deployment
            | Self::LonghornNode
            | Self::Pod
            | Self::StatefulSet => true,
        }
    }

    /// Whether it is a workload, ready when its rollout is complete.
    #[must_use]
    pub const fn rollout(self) -> bool {
        match self {
            Self::DaemonSet | Self::Deployment | Self::StatefulSet => true,
            Self::ApiService | Self::GatewayClass | Self::LonghornNode | Self::Pod => false,
        }
    }
}

/// An e2e check after install.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Smoke {
    /// Its name.
    pub name: String,
    /// Objects to apply first, in the namespace `catalog-smoke`.
    #[serde(default)]
    pub manifest: Option<String>,
    /// What to wait for after the objects are applied.
    pub wait: Vec<Wait>,
    /// How long to wait; 180 seconds when absent.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::TIMEOUT)))]
    pub timeout: Option<String>,
}

/// One `kubectl wait` of a smoke check.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("allOf" = [{"if": {"properties": {"resource": {"pattern": crate::schema::pattern::ROLLOUT}}}, "then": {"properties": {"condition": {"pattern": "^jsonpath="}}}, "else": {"required": ["condition"]}}], "if" = {"properties": {"resource": {"pattern": "/"}}}, "then" = {"not": {"anyOf": [{"required": ["selector"]}, {"required": ["all"]}]}}, "else" = {"oneOf": [{"required": ["selector"]}, {"required": ["all"]}]})))]
pub struct Wait {
    /// One object as `kind/name`, or a kind alone with a selector or `all`.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::WAIT_RESOURCE)))]
    pub resource: String,
    /// The namespace of the objects.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub namespace: Option<String>,
    /// The labels of the objects.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("minProperties" = 1)))]
    pub selector: Option<BTreeMap<String, String>>,
    /// Every object of the kind.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("const" = true)))]
    pub all: bool,
    /// What the objects reach, as a `kubectl wait --for` value.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::CONDITION)))]
    pub condition: Option<String>,
}

/// The prefix of the releases that [`default_accounts`] adds.
pub const DEFAULT_ACCOUNT_PREFIX: &str = "default-account-";

/// One release per namespace of `component` outside `kube-system`, with a default
/// `ServiceAccount` that mounts no token.
///
/// A pod that names no account then reaches nothing in the API. An installer applies them before
/// the component's own releases.
#[must_use]
pub fn default_accounts(component: &Component) -> Vec<Release> {
    let mut namespaces: Vec<&str> = Vec::new();
    for release in &component.releases {
        let Some(namespace) = release.namespace() else {
            continue;
        };
        if namespace != "kube-system" && !namespaces.contains(&namespace) {
            namespaces.push(namespace);
        }
    }
    namespaces
        .into_iter()
        .map(|namespace| {
            Release::Inline(InlineRelease {
                name: format!("{DEFAULT_ACCOUNT_PREFIX}{namespace}"),
                namespace: namespace.to_owned(),
                skip_if_present: Vec::new(),
                manifests: format!(
                    "apiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: default\n  namespace: {namespace}\nautomountServiceAccountToken: false\n"
                ),
            })
        })
        .collect()
}
