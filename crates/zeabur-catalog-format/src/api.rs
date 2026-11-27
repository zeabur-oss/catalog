//! The static API of a catalog release [releases]: `catalog.json`, `components/<id>.json` and
//! `templates/<id>.json` under `/v1/`, and the measurement files they carry.
//!
//! [releases]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/releases.mdx

use std::collections::BTreeMap;

#[cfg(feature = "schema")]
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use crate::common::{Distribution, Profile, WorkloadKind};
use crate::component::Component;
use crate::template::Template;

/// `catalog.json`: the release, its test environments, its components and its templates.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Index {
    /// The release.
    pub catalog: CatalogRelease,
    /// The profiles every component describes.
    pub profiles: Vec<Profile>,
    /// The Kubernetes versions every profile is rendered against.
    pub kubernetes: Vec<String>,
    /// The release of each host distribution that e2e installs.
    pub distributions: Vec<PinnedDistribution>,
    /// Each component's id and digest.
    pub components: Vec<Entry>,
    /// Each template's id and digest.
    pub templates: Vec<Entry>,
}

impl Index {
    /// The release of `distribution` that e2e installs.
    #[must_use]
    pub fn pinned(&self, distribution: Distribution) -> Option<&str> {
        self.distributions
            .iter()
            .find(|d| d.name == distribution)
            .map(|d| d.version.as_str())
    }
}

/// The catalog release an API was built from.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct CatalogRelease {
    /// `0.5.6`.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub version: String,
    /// The Git commit.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::COMMIT)))]
    pub commit: String,
}

/// The release of a distribution that e2e installs on the host.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct PinnedDistribution {
    /// The distribution.
    pub name: Distribution,
    /// Its release, such as `v1.37.2+k3s1`.
    pub version: String,
}

/// A component or template in `catalog.json`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Entry {
    /// Its directory name.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::ID)))]
    pub id: String,
    /// The Go module hash (`h1:`) of its directory without its READMEs.
    pub source_digest: String,
}

/// `components/<id>.json`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct ComponentFile {
    /// Its id.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::ID)))]
    pub id: String,
    /// The digest of its directory.
    pub source_digest: String,
    /// `component.yaml` as written.
    #[cfg_attr(
        feature = "schema",
        schemars(schema_with = "crate::schema::component_ref")
    )]
    pub declaration: Component,
    /// What it renders on each profile.
    pub render: Render,
    /// Every recorded run of its e2e test.
    pub measurements: Vec<Environment>,
    /// The locales of its READMEs, such as `en` and `zh-TW`. Each is a Markdown file next to this
    /// one: `components/<id>/README.md` for `en`, `components/<id>/README.<locale>.md` for
    /// another.
    #[serde(default)]
    pub readmes: Vec<String>,
}

impl AsRef<Component> for ComponentFile {
    fn as_ref(&self) -> &Component {
        &self.declaration
    }
}

impl ComponentFile {
    /// Where the component passed its test [installers]: for each distribution `index` pins, the
    /// platforms whose newest passing run on the pinned release has this file's digest and is no
    /// older than the newest failed run there. A pass on the same day as a failure wins.
    ///
    /// [installers]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/installers.mdx#tested-on
    #[must_use]
    pub fn tested_on(&self, index: &Index) -> Vec<TestedOn> {
        index
            .distributions
            .iter()
            .filter(|pinned| pinned.name != Distribution::Unknown)
            .filter_map(|pinned| {
                let mut platforms: Vec<String> = Vec::new();
                for run in &self.measurements {
                    let platform = run.platform.to_string();
                    if run.distribution == MeasuredDistribution::Host(pinned.name)
                        && run.distribution_version.as_deref() == Some(pinned.version.as_str())
                        && !platforms.contains(&platform)
                        && self.passed_last(pinned, &platform)
                    {
                        platforms.push(platform);
                    }
                }
                (!platforms.is_empty()).then_some(TestedOn {
                    distribution: pinned.name,
                    platforms,
                })
            })
            .collect()
    }

    fn passed_last(&self, pinned: &PinnedDistribution, platform: &str) -> bool {
        let runs = self.measurements.iter().filter(|m| {
            m.distribution == MeasuredDistribution::Host(pinned.name)
                && m.distribution_version.as_deref() == Some(pinned.version.as_str())
                && m.platform.to_string() == platform
        });
        let (mut pass, mut fail): (Option<&Environment>, Option<&str>) = (None, None);
        for run in runs {
            match run.result {
                RunResult::Passed => {
                    if pass.is_none_or(|p| run.measured_at > p.measured_at) {
                        pass = Some(run);
                    }
                }
                RunResult::Failed | RunResult::Unknown => {
                    if fail.is_none_or(|f| run.measured_at.as_str() > f) {
                        fail = Some(&run.measured_at);
                    }
                }
            }
        }
        pass.is_some_and(|p| {
            p.source_digest.as_deref() == Some(self.source_digest.as_str())
                && fail.is_none_or(|f| p.measured_at.as_str() >= f)
        })
    }
}

/// A distribution where a component passed its test, with the platforms it passed on.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct TestedOn {
    /// The distribution.
    pub distribution: Distribution,
    /// The platforms, such as `linux/amd64`.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    #[cfg_attr(feature = "schema", schemars(extend("uniqueItems" = true)))]
    #[cfg_attr(feature = "schema", schemars(inner(regex(pattern = crate::schema::pattern::PLATFORM))))]
    pub platforms: Vec<String>,
}

/// What `helm template` renders for each profile with the test inputs and every contribution.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Render {
    /// The Kubernetes version of the render.
    pub kubernetes_version: String,
    /// The objects of each profile. A profile where the component is unsupported is missing.
    pub profiles: BTreeMap<Profile, RenderedProfile>,
}

/// The objects of a component on one profile.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct RenderedProfile {
    /// Its workloads.
    pub workloads: Vec<RenderedWorkload>,
    /// Its `PersistentVolumeClaims`.
    pub persistent_volume_claims: Vec<RenderedClaim>,
}

/// A rendered workload. One that an operator creates later is missing.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct RenderedWorkload {
    /// The release that renders it.
    pub release: String,
    /// Its namespace.
    pub namespace: String,
    /// Its name.
    pub name: String,
    /// Its kind.
    pub kind: WorkloadKind,
    /// `spec.replicas`; a `DaemonSet` has none.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub replicas: Option<i32>,
    /// Its containers.
    pub containers: Vec<RenderedContainer>,
    /// Its init containers.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub init_containers: Vec<RenderedContainer>,
    /// The claims a `StatefulSet` makes for each pod.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub volume_claim_templates: Vec<RenderedClaim>,
}

/// The requests and limits of a container, by resource name.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct RenderedContainer {
    /// Its name.
    pub name: String,
    /// Its requests.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    #[cfg_attr(feature = "schema", schemars(extend("additionalProperties" = {"type": "string", "pattern": crate::schema::pattern::QUANTITY})))]
    pub requests: BTreeMap<String, String>,
    /// Its limits.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    #[cfg_attr(feature = "schema", schemars(extend("additionalProperties" = {"type": "string", "pattern": crate::schema::pattern::QUANTITY})))]
    pub limits: BTreeMap<String, String>,
}

/// The requested storage of a `PersistentVolumeClaim`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct RenderedClaim {
    /// The release that renders it; a `volumeClaimTemplates` entry has none.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub release: Option<String>,
    /// Its namespace; a `volumeClaimTemplates` entry has none.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub namespace: Option<String>,
    /// Its name.
    pub name: String,
    /// The storage it requests.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::QUANTITY)))]
    pub storage: String,
}

/// `templates/<id>.json`: the two files of a template as written.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct TemplateFile {
    /// Its id.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::ID)))]
    pub id: String,
    /// The digest of its directory.
    pub source_digest: String,
    /// `template.yaml` as written.
    #[cfg_attr(
        feature = "schema",
        schemars(schema_with = "crate::schema::template_ref")
    )]
    pub template: Template,
    /// The text of `manifests.yaml`, with template fields such as `{{ .Namespace }}`.
    pub manifests: String,
    /// Every recorded run of its e2e test. A run tested this template when its `sourceDigest` is
    /// this file's.
    #[serde(default)]
    pub measurements: Vec<Environment>,
    /// The locales of its READMEs, such as `en` and `zh-TW`. Each is a Markdown file next to this
    /// one: `templates/<id>/README.md` for `en`, `templates/<id>/README.<locale>.md` for another.
    #[serde(default)]
    pub readmes: Vec<String>,
}

/// `measurements/components/<id>.json` or `measurements/templates/<id>.json`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct MeasurementFile {
    /// The component's id; a template's file has none.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub component: Option<String>,
    /// The template's id; a component's file has none.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub template: Option<String>,
    /// Every recorded run.
    pub environments: Vec<Environment>,
}

/// One run of an e2e test.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Environment {
    /// Where it ran.
    pub distribution: MeasuredDistribution,
    /// The profile whose values it installed.
    pub profile: Profile,
    /// The platform of the nodes.
    pub platform: Platform,
    /// Absent when the cluster never answered.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub kubernetes_version: Option<String>,
    /// The release of a host distribution; kind has none.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub distribution_version: Option<String>,
    /// Whether it passed.
    pub result: RunResult,
    /// The idle use of one pod of each workload it started.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub pods: Vec<PodUsage>,
    /// The version of each release or image it installed.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub versions: Vec<MeasuredVersion>,
    /// The digest of the directory it installed.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source_digest: Option<String>,
    /// The digest of each component it installed first, by id.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub dependencies: BTreeMap<String, String>,
    /// The pinned release of each add-on it installed on the host, by name.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub add_ons: BTreeMap<String, String>,
    /// The day, `YYYY-MM-DD` in UTC.
    #[cfg_attr(feature = "schema", schemars(extend("format" = "date")))]
    pub measured_at: String,
}

/// Where an e2e run ran: kind, or a host distribution.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(untagged)]
pub enum MeasuredDistribution {
    /// kind.
    Kind(Kind),
    /// A distribution installed on the host.
    Host(Distribution),
}

/// kind, Kubernetes in Docker.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "lowercase")]
pub enum Kind {
    /// `kind`.
    Kind,
}

/// Whether a run passed.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "lowercase")]
pub enum RunResult {
    /// It passed.
    Passed,
    /// It failed.
    Failed,
    /// A result this reader does not know, read as a failed run.
    #[serde(other)]
    #[cfg_attr(feature = "schema", schemars(skip))]
    Unknown,
}

/// An OS and an architecture.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Platform {
    /// `linux`.
    pub os: String,
    /// `amd64`.
    pub architecture: String,
}

impl std::fmt::Display for Platform {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}/{}", self.os, self.architecture)
    }
}

/// The idle use of one pod of a workload.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct PodUsage {
    /// The controller the pod belongs to.
    pub workload: String,
    /// The controller's kind.
    pub kind: String,
    /// Memory, such as `22Mi`.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::QUANTITY)))]
    pub memory: String,
    /// CPU, such as `3m`.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::QUANTITY)))]
    pub cpu: String,
}

/// The version of a component's release or of a template's image.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct MeasuredVersion {
    /// The release.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub release: Option<String>,
    /// The image.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image: Option<String>,
    /// The version.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    /// The digest.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub digest: Option<String>,
}
