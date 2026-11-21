//! Types that component files, template files and the API share.

use std::collections::BTreeMap;
use std::fmt;

use serde::{Deserialize, Serialize};

/// The `apiVersion` of every component and template file.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ApiVersion {
    /// `catalog.zeabur.com/v1`.
    #[serde(rename = "catalog.zeabur.com/v1")]
    V1,
}

/// A deployment layout. Every component describes both.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
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
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case", deny_unknown_fields)]
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
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum Mode {
    /// On by default.
    Recommended,
    /// Off by default.
    Optional,
    /// Not installed on the profile.
    Unsupported,
}

/// A host distribution of Kubernetes that a component runs on.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Distribution {
    /// K3s.
    K3s,
    /// RKE2.
    Rke2,
    /// KubeSolo.
    Kubesolo,
}

impl Distribution {
    /// The name the files use (`k3s`).
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::K3s => "k3s",
            Self::Rke2 => "rke2",
            Self::Kubesolo => "kubesolo",
        }
    }
}

impl fmt::Display for Distribution {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// A Pod Security Standard level.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PodSecurity {
    /// `restricted`.
    Restricted,
    /// `baseline`.
    Baseline,
    /// `privileged`.
    Privileged,
}

/// A component by id, or a capability: one component that provides it.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum Reference {
    /// `{component: <id>}`.
    Component(String),
    /// `{capability: <name>}`.
    Capability(String),
}

/// A name and a summary in English, with the summary in other languages.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Summary {
    /// One line.
    pub summary: String,
}

/// Text in other languages, by BCP 47 tag (`zh-TW`). `config/locales.yaml` lists the tags.
pub type Locales<T> = BTreeMap<String, T>;

/// A key of a Secret.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SecretRef {
    /// The Secret's name.
    pub name: String,
    /// The key.
    pub key: String,
    /// The Secret's namespace.
    pub namespace: String,
}

/// How an installer makes a value instead of asking for it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", deny_unknown_fields)]
pub enum Generate {
    /// `bytes` random bytes in `encoding`.
    #[serde(rename = "random")]
    Random {
        /// How many, 12 to 64.
        bytes: u8,
        /// How they are written.
        encoding: Encoding,
    },
    /// A free address in the cluster's Service CIDR, for a Service's fixed `ClusterIP`.
    #[serde(rename = "clusterIP")]
    ClusterIp,
}

/// How random bytes are written.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Encoding {
    /// Lowercase hexadecimal.
    Hex,
    /// base64url without padding.
    Base64url,
}

/// A workload kind that the catalog recognises by its labels or its rollout.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub enum WorkloadKind {
    /// `DaemonSet`.
    DaemonSet,
    /// `Deployment`.
    Deployment,
    /// `StatefulSet`.
    StatefulSet,
}
