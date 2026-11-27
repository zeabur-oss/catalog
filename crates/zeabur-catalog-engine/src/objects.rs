//! What the engine writes on the objects of an install, and the objects of K3s's and RKE2's
//! helm-controller that it writes and reads [helm-controller-types].
//!
//! [helm-controller-types]: https://github.com/k3s-io/helm-controller/blob/v0.16.27/pkg/apis/helm.cattle.io/v1/types.go

use k8s_openapi::apimachinery::pkg::apis::meta::v1::ObjectMeta;
use k8s_openapi::{Metadata, NamespaceResourceScope, Resource};
use serde::{Deserialize, Serialize};

/// The label that marks what the engine installed, with [`MANAGED_BY_ZEABUR`].
pub const MANAGED_BY: &str = "app.kubernetes.io/managed-by";
/// Its value.
pub const MANAGED_BY_ZEABUR: &str = "zeabur";
/// The label naming the component an object belongs to.
pub const COMPONENT: &str = "catalog.zeabur.com/component";
/// The label naming the release an object belongs to.
pub const RELEASE: &str = "catalog.zeabur.com/release";
/// The annotation naming the component file's `sourceDigest` an object was written from.
pub const SOURCE_DIGEST: &str = "catalog.zeabur.com/source-digest";
/// The annotation naming the catalog release an object was written from.
pub const CATALOG_VERSION: &str = "catalog.zeabur.com/version";
/// Where `HelmChart`s go: the namespace K3s's and RKE2's helm-controller watch.
pub const CHART_NAMESPACE: &str = "kube-system";
/// The field manager the engine writes as.
pub const FIELD_MANAGER: &str = "zeabur";
/// The annotation on a `HelmChart` that keeps the given and generated values of its release's
/// `path` inputs, as a JSON object of input name to value, for the next install to reuse.
pub const INPUTS: &str = "catalog.zeabur.com/inputs";

/// A resource kube-apiserver serves, as discovery describes it, at the version kubectl picks
/// for it.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct ServedResource {
    /// The API group; empty for the core group.
    pub group: String,
    /// The version (`v1`).
    pub version: String,
    /// The plural resource name used in paths (`deployments`).
    pub resource: String,
    /// The kind (`Deployment`).
    pub kind: String,
    /// `true` for a namespaced resource.
    pub namespaced: bool,
    /// The verbs it supports (`get`, `list`, …).
    pub verbs: Vec<String>,
}

/// A K3s/RKE2 `HelmChart`, with the fields the engine writes and the status readiness reads
/// [helm-controller-types].
///
/// [helm-controller-types]: https://github.com/k3s-io/helm-controller/blob/v0.16.27/pkg/apis/helm.cattle.io/v1/types.go
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct HelmChart {
    /// Its metadata; `managedFields` tells when the engine last wrote it.
    #[serde(default)]
    pub metadata: ObjectMeta,
    /// What to install.
    #[serde(default)]
    pub spec: HelmChartSpec,
    /// Its status.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub status: Option<HelmChartStatus>,
}

impl Resource for HelmChart {
    const API_VERSION: &'static str = "helm.cattle.io/v1";
    const GROUP: &'static str = "helm.cattle.io";
    const KIND: &'static str = "HelmChart";
    const VERSION: &'static str = "v1";
    const URL_PATH_SEGMENT: &'static str = "helmcharts";
    type Scope = NamespaceResourceScope;
}

impl Metadata for HelmChart {
    type Ty = ObjectMeta;

    fn metadata(&self) -> &ObjectMeta {
        &self.metadata
    }

    fn metadata_mut(&mut self) -> &mut ObjectMeta {
        &mut self.metadata
    }
}

/// `HelmChart.spec`.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HelmChartSpec {
    /// The chart's name in `repo`, or an OCI reference.
    #[serde(default)]
    pub chart: String,
    /// The chart repository.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub repo: Option<String>,
    /// The chart version.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    /// The namespace of the release.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target_namespace: Option<String>,
    /// What helm-controller does when an install or upgrade fails.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub failure_policy: Option<FailurePolicy>,
    /// The values, as YAML.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub values_content: Option<String>,
}

/// `HelmChart.spec.failurePolicy`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum FailurePolicy {
    /// Uninstall and install again, helm-controller's default.
    Reinstall,
    /// Keep the failed release as it is.
    Abort,
}

/// A K3s/RKE2 `HelmChartConfig`, which changes the values of the `HelmChart` of the same name
/// [helm-controller-types].
///
/// [helm-controller-types]: https://github.com/k3s-io/helm-controller/blob/v0.16.27/pkg/apis/helm.cattle.io/v1/types.go
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct HelmChartConfig {
    /// Its metadata; `managedFields` tells when the engine last wrote it.
    #[serde(default)]
    pub metadata: ObjectMeta,
    /// The values.
    #[serde(default)]
    pub spec: HelmChartConfigSpec,
}

impl Resource for HelmChartConfig {
    const API_VERSION: &'static str = "helm.cattle.io/v1";
    const GROUP: &'static str = "helm.cattle.io";
    const KIND: &'static str = "HelmChartConfig";
    const VERSION: &'static str = "v1";
    const URL_PATH_SEGMENT: &'static str = "helmchartconfigs";
    type Scope = NamespaceResourceScope;
}

impl Metadata for HelmChartConfig {
    type Ty = ObjectMeta;

    fn metadata(&self) -> &ObjectMeta {
        &self.metadata
    }

    fn metadata_mut(&mut self) -> &mut ObjectMeta {
        &mut self.metadata
    }
}

/// `HelmChartConfig.spec`.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HelmChartConfigSpec {
    /// The values merged over the chart's, as YAML.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub values_content: Option<String>,
}

/// `HelmChart.status`.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HelmChartStatus {
    /// The Job that installs or upgrades it.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub job_name: Option<String>,
    /// `JobCreated` and `Failed`.
    #[serde(default)]
    pub conditions: Vec<Condition>,
}

/// A status condition.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Condition {
    /// Its type.
    #[serde(rename = "type")]
    pub kind: String,
    /// `True`, `False` or `Unknown`.
    pub status: String,
    /// Why, in words.
    #[serde(default)]
    pub message: Option<String>,
}
