//! Types that component files, template files and the API share.

use std::collections::BTreeMap;
use std::fmt;

use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use ts_rs::TS;

/// The `apiVersion` of every component and template file.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
pub enum ApiVersion {
    /// `catalog.zeabur.com/v1`.
    #[serde(rename = "catalog.zeabur.com/v1")]
    V1,
}

/// A deployment layout. Every component describes both.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize, JsonSchema, TS,
)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "kebab-case")]
pub enum Profile {
    /// One node.
    SingleNode,
    /// Three nodes or more.
    MultiNode,
}

impl Profile {
    /// Both profiles, in the order the catalog lists them.
    pub const ALL: [Self; 2] = [Self::SingleNode, Self::MultiNode];

    /// The name the files use (`single-node`).
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::SingleNode => "single-node",
            Self::MultiNode => "multi-node",
        }
    }
}

impl fmt::Display for Profile {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// A value for each profile.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "kebab-case")]
pub struct PerProfile<T> {
    /// On `single-node`.
    pub single_node: T,
    /// On `multi-node`.
    pub multi_node: T,
}

impl<T> PerProfile<T> {
    /// The value on `profile`.
    #[must_use]
    pub const fn get(&self, profile: Profile) -> &T {
        match profile {
            Profile::SingleNode => &self.single_node,
            Profile::MultiNode => &self.multi_node,
        }
    }
}

/// Whether a profile installs a component by default.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize, JsonSchema, TS,
)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub enum Mode {
    /// On by default.
    Recommended,
    /// Off by default.
    Optional,
    /// Not installed on the profile.
    Unsupported,
    /// A mode this reader does not know, read as `unsupported`.
    #[serde(other)]
    #[schemars(skip)]
    #[ts(skip)]
    Unknown,
}

impl PerProfile<Mode> {
    /// The mode on `profile`, with a mode this reader does not know read as `unsupported`.
    #[must_use]
    pub const fn mode(self, profile: Profile) -> Mode {
        match *self.get(profile) {
            Mode::Unknown | Mode::Unsupported => Mode::Unsupported,
            Mode::Recommended => Mode::Recommended,
            Mode::Optional => Mode::Optional,
        }
    }
}

/// A host distribution of Kubernetes that a component runs on.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize, JsonSchema, TS,
)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "lowercase")]
pub enum Distribution {
    /// K3s.
    K3s,
    /// RKE2.
    Rke2,
    /// KubeSolo.
    Kubesolo,
    /// A distribution this reader does not know. An entry that names it is ignored.
    #[serde(other)]
    #[schemars(skip)]
    #[ts(skip)]
    Unknown,
}

impl Distribution {
    /// The name the files use (`k3s`).
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::K3s => "k3s",
            Self::Rke2 => "rke2",
            Self::Kubesolo => "kubesolo",
            Self::Unknown => "unknown",
        }
    }
}

impl fmt::Display for Distribution {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// A Pod Security Standard level.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize, JsonSchema, TS,
)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "lowercase")]
pub enum PodSecurity {
    /// `restricted`.
    Restricted,
    /// `baseline`.
    Baseline,
    /// `privileged`.
    Privileged,
    /// A level this reader does not know. A component or template with it is not offered.
    #[serde(other)]
    #[schemars(skip)]
    #[ts(skip)]
    Unknown,
}

impl PodSecurity {
    /// The name the files use (`restricted`).
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Restricted => "restricted",
            Self::Baseline => "baseline",
            Self::Privileged => "privileged",
            Self::Unknown => "unknown",
        }
    }
}

/// A component by id, or a capability: one component that provides it.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub enum Reference {
    /// `{component: <id>}`.
    #[schemars(schema_with = "crate::schema::id")]
    Component(String),
    /// `{capability: <name>}`.
    #[schemars(schema_with = "crate::schema::name")]
    Capability(String),
}

/// A name and a summary in English, with the summary in other languages.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct Summary {
    /// One line.
    #[schemars(regex(pattern = crate::schema::pattern::LINE))]
    pub summary: String,
}

/// Text in other languages, by BCP 47 tag (`zh-TW`). `config/locales.yaml` lists the tags.
pub type Locales<T> = BTreeMap<String, T>;

/// The URL of each `LicenseRef-` licence's text, by its id in `license`.
pub type LicenseUrls = BTreeMap<String, String>;

/// What is wrong with the link of a `LicenseRef-` id.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LicenseUrlProblem {
    /// `license` uses the id, and `licenseUrls` has no URL for it.
    Unlinked {
        /// The `LicenseRef-` id.
        id: String,
    },
    /// `licenseUrls` has a URL for the id, and `license` does not use it.
    Unused {
        /// The `LicenseRef-` id.
        id: String,
    },
}

impl fmt::Display for LicenseUrlProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Unlinked { id } => write!(f, "license {id} has no URL in licenseUrls"),
            Self::Unused { id } => write!(f, "licenseUrls names {id}, which license does not use"),
        }
    }
}

/// What is wrong with the links of `license`'s `LicenseRef-` ids.
#[must_use]
pub fn license_url_problems(license: &str, urls: &LicenseUrls) -> Vec<LicenseUrlProblem> {
    let used: Vec<&str> = license
        .split(|c: char| c.is_whitespace() || c == '(' || c == ')')
        .map(|token| token.trim_end_matches('+'))
        .filter(|token| token.starts_with("LicenseRef-"))
        .collect();
    let unlinked =
        used.iter()
            .filter(|id| !urls.contains_key(**id))
            .map(|id| LicenseUrlProblem::Unlinked {
                id: (*id).to_owned(),
            });
    let unused = urls
        .keys()
        .filter(|id| !used.contains(&id.as_str()))
        .map(|id| LicenseUrlProblem::Unused { id: id.clone() });
    unlinked.chain(unused).collect()
}

/// A key of a Secret.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "camelCase")]
pub struct SecretRef {
    /// The Secret's name.
    pub name: String,
    /// The key.
    pub key: String,
    /// The Secret's namespace.
    pub namespace: String,
}

/// How an installer makes a value instead of asking for it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(tag = "type")]
pub enum Generate {
    /// `bytes` random bytes in `encoding`.
    #[serde(rename = "random")]
    Random {
        /// How many, 12 to 64.
        #[schemars(range(min = 12, max = 64))]
        bytes: u8,
        /// How they are written.
        encoding: Encoding,
    },
    /// A free address in the cluster's Service CIDR, for a Service's fixed `ClusterIP`.
    #[serde(rename = "clusterIP")]
    ClusterIp,
    /// A type this reader does not know. The input is not filled in.
    #[serde(other)]
    #[schemars(skip)]
    #[ts(skip)]
    Unknown,
}

/// How random bytes are written.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema, TS)]
#[ts(export_to = "catalog.ts")]
#[serde(rename_all = "lowercase")]
pub enum Encoding {
    /// Lowercase hexadecimal.
    Hex,
    /// base64url without padding.
    Base64url,
}

/// A workload kind that the catalog recognises by its labels or its rollout.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize, JsonSchema, TS,
)]
#[ts(export_to = "catalog.ts")]
pub enum WorkloadKind {
    /// `DaemonSet`.
    DaemonSet,
    /// `Deployment`.
    Deployment,
    /// `StatefulSet`.
    StatefulSet,
    /// A kind this reader does not know. An entry that names it is ignored.
    #[serde(other)]
    #[schemars(skip)]
    #[ts(skip)]
    Unknown,
}

#[cfg(test)]
mod tests {
    use super::{LicenseUrlProblem, LicenseUrls, license_url_problems};

    fn urls(ids: &[&str]) -> LicenseUrls {
        ids.iter()
            .map(|id| ((*id).to_owned(), "https://example.com/license".to_owned()))
            .collect()
    }

    #[test]
    fn every_license_ref_needs_a_url() {
        let license = "(MIT OR LicenseRef-A) AND LicenseRef-B+";
        assert_eq!(
            license_url_problems(license, &urls(&["LicenseRef-A", "LicenseRef-B"])),
            []
        );
        assert_eq!(
            license_url_problems(license, &urls(&["LicenseRef-A"])),
            [LicenseUrlProblem::Unlinked {
                id: "LicenseRef-B".to_owned()
            }]
        );
    }

    #[test]
    fn a_url_needs_its_license_ref() {
        assert_eq!(
            license_url_problems("MIT", &urls(&["LicenseRef-A"])),
            [LicenseUrlProblem::Unused {
                id: "LicenseRef-A".to_owned()
            }]
        );
    }
}
