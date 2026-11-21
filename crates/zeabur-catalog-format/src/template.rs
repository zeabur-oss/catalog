//! `templates/<id>/template.yaml` [template-format]. Its objects are in `manifests.yaml` next to
//! it.
//!
//! [template-format]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-format.mdx

use serde::{Deserialize, Serialize};

use crate::common::{ApiVersion, Generate, Locales, PodSecurity, Reference, Summary};

/// The `kind` of a template file.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TemplateKind {
    /// `Template`.
    Template,
}

/// One application that a project member installs into one namespace.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
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
    pub requires: Vec<Reference>,
    /// The workloads in `manifests.yaml`.
    pub services: Vec<Service>,
    /// Values a person gives or the installer generates.
    #[serde(default)]
    pub inputs: Vec<Input>,
    /// What it exposes and how its risks are mitigated.
    pub security: Security,
    /// e2e checks after install, in the template's namespace.
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
#[serde(rename_all = "camelCase")]
pub struct Metadata {
    /// The name for people.
    pub name: String,
    /// What the application is, in one plain sentence.
    pub summary: String,
    /// The summary in other languages.
    pub locales: Locales<Summary>,
    /// An SPDX expression for the application, its images and charts.
    pub license: String,
    /// The project's home page.
    pub homepage: String,
    /// The upstream docs on how to use the application.
    pub docs: String,
    /// The tag of its main image.
    pub version: String,
}

/// A workload in `manifests.yaml`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Service {
    /// Its name.
    pub name: String,
    /// The level its pods meet.
    pub pod_security: PodSecurity,
    /// Its named ports.
    #[serde(default)]
    pub ports: Vec<Port>,
    /// Its requests and limits.
    pub resources: ServiceResources,
    /// Public hostnames; rendering then needs a domain and a `ClusterIssuer`.
    #[serde(default)]
    pub domains: Vec<String>,
}

/// A named port of a service.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Port {
    /// Its name.
    pub name: String,
    /// Its number.
    pub port: u16,
}

/// The requests and limits of a service.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ServiceResources {
    /// What it requests.
    pub requests: CpuMemory,
    /// What it may use.
    pub limits: CpuMemory,
}

/// A CPU and a memory quantity.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CpuMemory {
    /// CPU, such as `50m`.
    pub cpu: String,
    /// Memory, such as `64Mi`.
    pub memory: String,
}

/// A value the person installing gives, or the installer generates into a Secret.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Input {
    /// Its name, camelCase.
    pub name: String,
    /// What it is, in English.
    pub description: String,
    /// A short name for the field, in English.
    #[serde(default)]
    pub label: Option<String>,
    /// The label and description in other languages.
    #[serde(default)]
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
#[serde(rename_all = "camelCase")]
pub struct InputLocale {
    /// A short name for the field.
    pub label: String,
    /// What it is.
    pub description: String,
}

/// A key of a Secret in the template's namespace.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InputSecret {
    /// The Secret's name.
    pub name: String,
    /// The key.
    pub key: String,
}

/// What an input's value must match.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Validation {
    /// A regular expression the whole value matches.
    pub pattern: String,
}

/// What a template exposes and how its risks are mitigated.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Security {
    /// What it exposes.
    pub summary: String,
    /// The summary in other languages.
    #[serde(default)]
    pub locales: Locales<Summary>,
    /// Its risks.
    pub risks: Vec<Risk>,
    /// Services with no sign-in or key of their own.
    #[serde(default)]
    pub unauthenticated: Vec<String>,
}

/// A risk of a template and its mitigation.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
#[expect(clippy::struct_field_names, reason = "the file's key is `risk`")]
pub struct Risk {
    /// The risk.
    pub risk: String,
    /// How it is mitigated.
    pub mitigation: String,
    /// Both in other languages.
    pub locales: Locales<RiskLocale>,
}

/// A risk and its mitigation in one language.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RiskLocale {
    /// The risk.
    pub risk: String,
    /// How it is mitigated.
    pub mitigation: String,
}

/// An e2e check after install.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Smoke {
    /// Its name.
    pub name: String,
    /// The object to wait for, as `kind/name`.
    pub resource: String,
    /// What it reaches, as a `kubectl wait --for` value.
    #[serde(default)]
    pub condition: Option<String>,
    /// How long to wait.
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
#[serde(rename_all = "camelCase")]
pub struct HttpCheck {
    /// The Service.
    pub service: String,
    /// Its port.
    pub port: u16,
    /// The path, from `/`.
    pub path: String,
    /// The status it answers with.
    pub status: u16,
    /// Text the body contains.
    #[serde(default)]
    pub contains: Option<String>,
}

/// One read-only `SELECT` on a `CloudNativePG` primary.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SqlCheck {
    /// The `CloudNativePG` cluster.
    pub cluster: String,
    /// The database.
    pub database: String,
    /// The query.
    pub query: String,
    /// What it prints.
    pub expect: String,
}
