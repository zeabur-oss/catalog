//! `templates/<id>/template.yaml` [template-format]. Its objects are in `manifests.yaml` next to
//! it.
//!
//! [template-format]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-format.mdx

use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use ts_rs::TS;

use crate::common::{ApiVersion, Generate, Locales, PodSecurity, Reference, Summary};

/// The `kind` of a template file.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
pub enum TemplateKind {
    /// `Template`.
    Template,
}

/// One application that a project member installs into one namespace.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
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
    #[schemars(extend("uniqueItems" = true))]
    #[ts(as = "Option<_>", optional)]
    pub requires: Vec<Reference>,
    /// The workloads in `manifests.yaml`.
    #[schemars(length(min = 1))]
    pub services: Vec<Service>,
    /// Values a person gives or the installer generates.
    #[serde(default)]
    #[ts(as = "Option<_>", optional)]
    pub inputs: Vec<Input>,
    /// What it exposes and how its risks are mitigated.
    pub security: Security,
    /// e2e checks after install, in the template's namespace.
    #[schemars(length(min = 1))]
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
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts", rename = "TemplateMetadata")]
#[schemars(rename = "TemplateMetadata")]
#[serde(rename_all = "camelCase")]
pub struct Metadata {
    /// The name for people.
    #[schemars(length(min = 1))]
    pub name: String,
    /// What the application is, in one plain sentence.
    #[schemars(regex(pattern = crate::schema::pattern::LINE))]
    pub summary: String,
    /// The summary in other languages.
    #[schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE}))]
    pub locales: Locales<Summary>,
    /// An SPDX expression for the application, its images and charts.
    #[schemars(length(min = 1))]
    pub license: String,
    /// The project's home page.
    #[schemars(url)]
    #[schemars(regex(pattern = crate::schema::pattern::HTTP))]
    pub homepage: String,
    /// The upstream docs on how to use the application.
    #[schemars(url)]
    #[schemars(regex(pattern = crate::schema::pattern::HTTPS))]
    pub docs: String,
    /// The tag of its main image.
    pub version: String,
}

/// A workload in `manifests.yaml`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct Service {
    /// Its name.
    #[schemars(regex(pattern = crate::schema::pattern::SERVICE))]
    #[schemars(length(max = 53))]
    pub name: String,
    /// The level its pods meet.
    #[schemars(extend("enum" = ["restricted", "baseline"]))]
    pub pod_security: PodSecurity,
    /// Its named ports.
    #[serde(default)]
    #[ts(as = "Option<_>", optional)]
    pub ports: Vec<Port>,
    /// Its requests and limits.
    pub resources: ServiceResources,
    /// Public hostnames; rendering then needs a domain and a `ClusterIssuer`.
    #[serde(default)]
    #[schemars(inner(length(min = 1)))]
    #[ts(as = "Option<_>", optional)]
    pub domains: Vec<String>,
}

/// A named port of a service.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct Port {
    /// Its name.
    #[schemars(length(min = 1))]
    pub name: String,
    /// Its number.
    #[schemars(range(min = 1))]
    pub port: u16,
}

/// The requests and limits of a service.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct ServiceResources {
    /// What it requests.
    pub requests: CpuMemory,
    /// What it may use.
    pub limits: CpuMemory,
}

/// A CPU and a memory quantity.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct CpuMemory {
    /// CPU, such as `50m`.
    #[schemars(regex(pattern = crate::schema::pattern::CPU))]
    pub cpu: String,
    /// Memory, such as `64Mi`.
    #[schemars(regex(pattern = crate::schema::pattern::MEMORY))]
    pub memory: String,
}

/// A value the person installing gives, or the installer generates into a Secret.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts", rename = "TemplateInput")]
#[schemars(rename = "TemplateInput")]
#[serde(rename_all = "camelCase")]
#[schemars(extend("oneOf" = [{"not": {"anyOf": [{"required": ["required"]}, {"required": ["default"]}]}, "required": ["generate", "secret"]}, {"not": {"anyOf": [{"required": ["generate"]}, {"required": ["secret"]}]}, "required": ["required", "label", "locales"]}]))]
pub struct Input {
    /// Its name, camelCase.
    #[schemars(regex(pattern = crate::schema::pattern::INPUT))]
    pub name: String,
    /// What it is, in English.
    #[schemars(length(min = 1))]
    pub description: String,
    /// A short name for the field, in English.
    #[serde(default)]
    #[schemars(length(max = 40))]
    #[ts(optional)]
    pub label: Option<String>,
    /// The label and description in other languages.
    #[serde(default)]
    #[schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE}))]
    #[ts(as = "Option<_>", optional)]
    pub locales: Locales<InputLocale>,
    /// How the installer makes it.
    #[serde(default)]
    #[ts(optional)]
    pub generate: Option<Generate>,
    /// A deploy needs a value.
    #[serde(default)]
    #[ts(as = "Option<_>", optional)]
    pub required: bool,
    /// The value when a deploy does not give it.
    #[serde(default)]
    #[ts(optional)]
    pub default: Option<String>,
    /// The Secret key a generated value is stored in.
    #[serde(default)]
    #[ts(optional)]
    pub secret: Option<InputSecret>,
    /// What the value must match.
    pub validation: Validation,
}

/// An input's label and description in one language.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct InputLocale {
    /// A short name for the field.
    #[schemars(length(max = 40))]
    pub label: String,
    /// What it is.
    #[schemars(length(min = 1))]
    pub description: String,
}

/// A key of a Secret in the template's namespace.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct InputSecret {
    /// The Secret's name.
    #[schemars(regex(pattern = crate::schema::pattern::SECRET_NAME))]
    pub name: String,
    /// The key.
    #[schemars(regex(pattern = crate::schema::pattern::SECRET_KEY))]
    pub key: String,
}

/// What an input's value must match.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct Validation {
    /// A regular expression the whole value matches.
    #[schemars(length(min = 1))]
    pub pattern: String,
}

/// What a template exposes and how its risks are mitigated.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts", rename = "TemplateSecurity")]
#[schemars(rename = "TemplateSecurity")]
#[serde(rename_all = "camelCase")]
pub struct Security {
    /// What it exposes.
    #[schemars(length(min = 1))]
    pub summary: String,
    /// The summary in other languages.
    #[serde(default)]
    #[schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE}))]
    #[ts(as = "Option<_>", optional)]
    pub locales: Locales<Summary>,
    /// Its risks.
    pub risks: Vec<Risk>,
    /// Services with no sign-in or key of their own.
    #[serde(default)]
    #[schemars(extend("uniqueItems" = true))]
    #[ts(as = "Option<_>", optional)]
    pub unauthenticated: Vec<String>,
}

/// A risk of a template and its mitigation.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
#[expect(clippy::struct_field_names, reason = "the file's key is `risk`")]
pub struct Risk {
    /// The risk.
    #[schemars(length(min = 1))]
    pub risk: String,
    /// How it is mitigated.
    #[schemars(length(min = 1))]
    pub mitigation: String,
    /// Both in other languages.
    #[schemars(extend("propertyNames" = {"type": "string", "pattern": crate::schema::pattern::LOCALE}))]
    pub locales: Locales<RiskLocale>,
}

/// A risk and its mitigation in one language.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct RiskLocale {
    /// The risk.
    #[schemars(length(min = 1))]
    pub risk: String,
    /// How it is mitigated.
    #[schemars(length(min = 1))]
    pub mitigation: String,
}

/// An e2e check after install.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts", rename = "TemplateSmoke")]
#[schemars(rename = "TemplateSmoke")]
#[serde(rename_all = "camelCase")]
#[schemars(extend("if" = {"properties": {"resource": {"pattern": crate::schema::pattern::ROLLOUT}}}, "then" = {"not": {"required": ["condition"]}}, "else" = {"required": ["condition"]}))]
pub struct Smoke {
    /// Its name.
    #[schemars(length(min = 1))]
    pub name: String,
    /// The object to wait for, as `kind/name`.
    #[schemars(regex(pattern = crate::schema::pattern::TEMPLATE_RESOURCE))]
    pub resource: String,
    /// What it reaches, as a `kubectl wait --for` value.
    #[serde(default)]
    #[schemars(regex(pattern = crate::schema::pattern::CONDITION))]
    #[ts(optional)]
    pub condition: Option<String>,
    /// How long to wait.
    #[schemars(regex(pattern = crate::schema::pattern::TIMEOUT))]
    pub timeout: String,
    /// A GET through a port-forward to a Service.
    #[serde(default)]
    #[ts(optional)]
    pub http: Option<HttpCheck>,
    /// One read-only `SELECT` on a `CloudNativePG` primary.
    #[serde(default)]
    #[ts(optional)]
    pub sql: Option<SqlCheck>,
}

/// A GET through a port-forward to a Service.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct HttpCheck {
    /// The Service.
    #[schemars(regex(pattern = crate::schema::pattern::SERVICE_REF))]
    pub service: String,
    /// Its port.
    #[schemars(range(min = 1))]
    pub port: u16,
    /// The path, from `/`.
    #[schemars(regex(pattern = crate::schema::pattern::PATH))]
    pub path: String,
    /// The status it answers with.
    #[schemars(range(min = 100, max = 599))]
    pub status: u16,
    /// Text the body contains.
    #[serde(default)]
    #[ts(optional)]
    pub contains: Option<String>,
}

/// One read-only `SELECT` on a `CloudNativePG` primary.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct SqlCheck {
    /// The `CloudNativePG` cluster.
    #[schemars(regex(pattern = crate::schema::pattern::SERVICE))]
    pub cluster: String,
    /// The database.
    #[schemars(regex(pattern = crate::schema::pattern::DATABASE))]
    pub database: String,
    /// The query.
    #[schemars(regex(pattern = crate::schema::pattern::SELECT))]
    pub query: String,
    /// What it prints.
    pub expect: String,
}
