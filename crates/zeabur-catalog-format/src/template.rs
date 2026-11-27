//! `templates/<id>/template.yaml` [template-format]. Its objects are in `manifests.yaml` next to
//! it.
//!
//! [template-format]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-format.mdx

#[cfg(feature = "schema")]
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use crate::common::{ApiVersion, Generate, LicenseUrls, Locales, PodSecurity, Reference, Summary};

/// The `kind` of a template file.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
pub enum TemplateKind {
    /// `Template`.
    Template,
}

/// One application that a project member installs into one namespace.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Template {
    /// `catalog.zeabur.com/v1`.
    pub api_version: ApiVersion,
    /// `Template`.
    pub kind: TemplateKind,
    /// Its name, summary, version and docs.
    pub metadata: Metadata,
    /// The components the cluster must have.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("uniqueItems" = true)))]
    pub requires: Vec<Reference>,
    /// The workloads in `manifests.yaml`.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub services: Vec<Service>,
    /// Values a person gives or the installer generates.
    #[serde(default)]
    pub inputs: Vec<Input>,
    /// What it exposes and how its risks are mitigated.
    pub security: Security,
    /// e2e checks after install, in the template's namespace.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub smoke: Vec<Smoke>,
}

impl Template {
    /// Whether this reader can install it: the pod security level of every service and the
    /// `generate` type of every required input without a default are known. One it cannot
    /// install is not offered.
    #[must_use]
    pub fn known(&self) -> bool {
        !self
            .services
            .iter()
            .any(|s| s.pod_security == PodSecurity::Unknown)
            && !self
                .inputs
                .iter()
                .any(|i| i.required && i.default.is_none() && i.generate == Some(Generate::Unknown))
    }
}

/// A template's name, summary, version and docs.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[cfg_attr(feature = "schema", schemars(rename = "TemplateMetadata"))]
#[serde(rename_all = "camelCase")]
pub struct Metadata {
    /// The name for people.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub name: String,
    /// What the application is, in one plain sentence.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::LINE)))]
    pub summary: String,
    /// The summary in other languages.
    #[cfg_attr(feature = "schema", schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE})))]
    pub locales: Locales<Summary>,
    /// An SPDX expression for the application, its images and charts.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
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
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::HTTP)))]
    pub homepage: String,
    /// The upstream docs on how to use the application.
    #[cfg_attr(feature = "schema", schemars(url))]
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::HTTPS)))]
    pub docs: String,
    /// The tag of its main image.
    pub version: String,
}

/// A workload in `manifests.yaml`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Service {
    /// Its name.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::SERVICE)))]
    #[cfg_attr(feature = "schema", schemars(length(max = 53)))]
    pub name: String,
    /// The level its pods meet.
    #[cfg_attr(feature = "schema", schemars(extend("enum" = ["restricted", "baseline"])))]
    pub pod_security: PodSecurity,
    /// Its named ports.
    #[serde(default)]
    pub ports: Vec<Port>,
    /// Its requests and limits.
    pub resources: ServiceResources,
    /// Public hostnames; rendering then needs a domain and a `ClusterIssuer`.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(inner(length(min = 1))))]
    pub domains: Vec<String>,
}

/// A named port of a service.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Port {
    /// Its name.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub name: String,
    /// Its number.
    #[cfg_attr(feature = "schema", schemars(range(min = 1)))]
    pub port: u16,
}

/// The requests and limits of a service.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct ServiceResources {
    /// What it requests.
    pub requests: CpuMemory,
    /// What it may use.
    pub limits: CpuMemory,
}

/// A CPU and a memory quantity.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct CpuMemory {
    /// CPU, such as `50m`.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::CPU)))]
    pub cpu: String,
    /// Memory, such as `64Mi`.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::MEMORY)))]
    pub memory: String,
}

/// A value the person installing gives, or the installer generates into a Secret.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[cfg_attr(feature = "schema", schemars(rename = "TemplateInput"))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("oneOf" = [{"not": {"anyOf": [{"required": ["required"]}, {"required": ["default"]}]}, "required": ["generate", "secret"]}, {"not": {"anyOf": [{"required": ["generate"]}, {"required": ["secret"]}]}, "required": ["required", "label", "locales"]}])))]
pub struct Input {
    /// Its name, camelCase.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::INPUT)))]
    pub name: String,
    /// What it is, in English.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub description: String,
    /// A short name for the field, in English.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(length(max = 40)))]
    pub label: Option<String>,
    /// The label and description in other languages.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE})))]
    pub locales: Locales<InputLocale>,
    /// How the installer makes it.
    #[serde(default)]
    pub generate: Option<Generate>,
    /// A deploy needs a value.
    #[serde(default)]
    pub required: bool,
    /// The value when a deploy does not give it.
    #[serde(default)]
    pub default: Option<String>,
    /// The Secret key a generated value is stored in.
    #[serde(default)]
    pub secret: Option<InputSecret>,
    /// What the value must match.
    pub validation: Validation,
}

/// An input's label and description in one language.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct InputLocale {
    /// A short name for the field.
    #[cfg_attr(feature = "schema", schemars(length(max = 40)))]
    pub label: String,
    /// What it is.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub description: String,
}

/// A key of a Secret in the template's namespace.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct InputSecret {
    /// The Secret's name.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::SECRET_NAME)))]
    pub name: String,
    /// The key.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::SECRET_KEY)))]
    pub key: String,
}

/// What an input's value must match.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct Validation {
    /// A regular expression the whole value matches.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub pattern: String,
}

/// What a template exposes and how its risks are mitigated.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[cfg_attr(feature = "schema", schemars(rename = "TemplateSecurity"))]
#[serde(rename_all = "camelCase")]
pub struct Security {
    /// What it exposes.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub summary: String,
    /// The summary in other languages.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE})))]
    pub locales: Locales<Summary>,
    /// Its risks.
    pub risks: Vec<Risk>,
    /// Services with no sign-in or key of their own.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(extend("uniqueItems" = true)))]
    pub unauthenticated: Vec<String>,
}

/// A risk of a template and its mitigation.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
#[expect(clippy::struct_field_names, reason = "the file's key is `risk`")]
pub struct Risk {
    /// The risk.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub risk: String,
    /// How it is mitigated.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub mitigation: String,
    /// Both in other languages.
    #[cfg_attr(feature = "schema", schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE})))]
    pub locales: Locales<RiskLocale>,
}

/// A risk and its mitigation in one language.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct RiskLocale {
    /// The risk.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub risk: String,
    /// How it is mitigated.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub mitigation: String,
}

/// An e2e check after install.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[cfg_attr(feature = "schema", schemars(rename = "TemplateSmoke"))]
#[serde(rename_all = "camelCase")]
#[cfg_attr(feature = "schema", schemars(extend("if" = {"properties": {"resource": {"pattern": crate::schema::pattern::ROLLOUT}}}, "then" = {"not": {"required": ["condition"]}}, "else" = {"required": ["condition"]})))]
pub struct Smoke {
    /// Its name.
    #[cfg_attr(feature = "schema", schemars(length(min = 1)))]
    pub name: String,
    /// The object to wait for, as `kind/name`.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::TEMPLATE_RESOURCE)))]
    pub resource: String,
    /// What it reaches, as a `kubectl wait --for` value.
    #[serde(default)]
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::CONDITION)))]
    pub condition: Option<String>,
    /// How long to wait.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::TIMEOUT)))]
    pub timeout: String,
    /// A GET through a port-forward to a Service.
    #[serde(default)]
    pub http: Option<HttpCheck>,
    /// One read-only `SELECT` on a `CloudNativePG` primary.
    #[serde(default)]
    pub sql: Option<SqlCheck>,
}

/// A GET through a port-forward to a Service.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct HttpCheck {
    /// The Service.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::SERVICE_REF)))]
    pub service: String,
    /// Its port.
    #[cfg_attr(feature = "schema", schemars(range(min = 1)))]
    pub port: u16,
    /// The path, from `/`.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::PATH)))]
    pub path: String,
    /// The status it answers with.
    #[cfg_attr(feature = "schema", schemars(range(min = 100, max = 599)))]
    pub status: u16,
    /// Text the body contains.
    #[serde(default)]
    pub contains: Option<String>,
}

/// One read-only `SELECT` on a `CloudNativePG` primary.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "schema", derive(JsonSchema))]
#[serde(rename_all = "camelCase")]
pub struct SqlCheck {
    /// The `CloudNativePG` cluster.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::SERVICE)))]
    pub cluster: String,
    /// The database.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::DATABASE)))]
    pub database: String,
    /// The query.
    #[cfg_attr(feature = "schema", schemars(regex(pattern = crate::schema::pattern::SELECT)))]
    pub query: String,
    /// What it prints.
    pub expect: String,
}
