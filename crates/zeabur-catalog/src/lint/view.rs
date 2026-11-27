//! How the lint reads a rendered object: as the fields it knows, where a field of another shape
//! reads as empty, and as the Kubernetes type an operator turns a map into.

use std::collections::{BTreeMap, BTreeSet};

use k8s_openapi::apimachinery::pkg::apis::meta::v1::ObjectMeta;
use serde::de::{DeserializeOwned, IgnoredAny};
use serde::{Deserialize, Deserializer};
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
use serde_json::Value;

/// A value as the field it should be, or the empty one when it has another shape: a list where
/// a map should be, or a number where text should be.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub(super) struct Loose<T>(pub(super) T);

#[expect(
    clippy::disallowed_types,
    reason = "a field of a custom resource may hold any shape; one the lint does not know reads \
              as empty"
)]
impl<'de, T: DeserializeOwned + Default> Deserialize<'de> for Loose<T> {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let value = Value::deserialize(deserializer)?;
        Ok(Self(T::deserialize(value).unwrap_or_default()))
    }
}

/// A field that should be a map, read three ways: its keys, the fields `V` of it the lint
/// knows, and the Kubernetes type `T` the operator turns it into. A value that is no map is
/// absent and converts to an empty `T`.
#[derive(Debug, Clone)]
pub(super) struct Obj<T, V = IgnoredAny> {
    /// Whether the value is a map.
    pub(super) present: bool,
    /// Its keys.
    pub(super) keys: Keys,
    /// The fields the lint knows.
    pub(super) view: V,
    /// The Kubernetes type, or why the map does not read as it.
    pub(super) typed: Result<T, String>,
}

impl<T: Default, V: Default> Default for Obj<T, V> {
    fn default() -> Self {
        Self {
            present: false,
            keys: Keys::default(),
            view: V::default(),
            typed: Ok(T::default()),
        }
    }
}

#[expect(
    clippy::disallowed_types,
    reason = "a field of a custom resource may hold any shape; the lint reads its keys and \
              converts it as the operator does"
)]
impl<'de, T: DeserializeOwned + Default, V: DeserializeOwned + Default> Deserialize<'de>
    for Obj<T, V>
{
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let value = Value::deserialize(deserializer)?;
        let Value::Object(map) = &value else {
            return Ok(Self::default());
        };
        Ok(Self {
            present: true,
            keys: Keys(map.keys().cloned().collect()),
            view: V::deserialize(&value).unwrap_or_default(),
            typed: T::deserialize(&value).map_err(|e| e.to_string()),
        })
    }
}

/// The keys of a map, in order, or none for another shape.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub(super) struct Keys(pub(super) BTreeSet<String>);

impl<'de> Deserialize<'de> for Keys {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let Loose(map) = Loose::<BTreeMap<String, IgnoredAny>>::deserialize(deserializer)?;
        Ok(Self(map.into_keys().collect()))
    }
}

/// Labels, or none for another shape. A value that is not text reads as empty.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub(super) struct Labels(BTreeMap<String, String>);

impl<'de> Deserialize<'de> for Labels {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let Loose(map) = Loose::<BTreeMap<String, Loose<String>>>::deserialize(deserializer)?;
        Ok(Self(map.into_iter().map(|(k, v)| (k, v.0)).collect()))
    }
}

impl Labels {
    /// The value of the label `key`, empty when it is not set.
    pub(super) fn label(&self, key: &str) -> &str {
        self.0.get(key).map_or("", String::as_str)
    }
}

/// A map with `labels`, such as a CloudNativePG `inheritedMetadata`.
#[derive(Debug, Default, Deserialize)]
pub(super) struct Labelled {
    #[serde(default)]
    pub(super) labels: Labels,
}

impl Labelled {
    /// The value of the label `key`, empty when it is not set.
    pub(super) fn label(&self, key: &str) -> &str {
        self.labels.label(key)
    }
}

/// The API version, kind and metadata of a rendered object, empty where they do not read.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(super) struct Head {
    #[serde(default)]
    pub(super) api_version: Loose<String>,
    #[serde(default)]
    pub(super) kind: Loose<String>,
    #[serde(default)]
    pub(super) metadata: Loose<ObjectMeta>,
}

impl Head {
    /// The name, empty when it has none.
    pub(super) fn name(&self) -> &str {
        self.metadata.0.name.as_deref().unwrap_or_default()
    }

    /// Whether it is `api_version` `kind` named `name`.
    pub(super) fn is(&self, api_version: &str, kind: &str, name: &str) -> bool {
        self.api_version.0 == api_version && self.kind.0 == kind && self.name() == name
    }
}

/// The `spec` of a rendered object, read as `T`.
#[derive(Debug, Default, Deserialize)]
pub(super) struct Spec<T> {
    #[serde(default)]
    pub(super) spec: T,
}

/// `doc` read as `T`, or an empty `T` when it is not an object.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
pub(super) fn view<T: DeserializeOwned + Default>(doc: &Value) -> T {
    T::deserialize(doc).unwrap_or_default()
}

/// Whether a map anywhere in `value` has one of `keys`.
#[expect(
    clippy::disallowed_types,
    reason = "the lint looks for a key in every field of an object of any kind"
)]
pub(super) fn has_key(value: &Value, keys: &[&str]) -> bool {
    match value {
        Value::Object(map) => map
            .iter()
            .any(|(k, v)| keys.contains(&k.as_str()) || has_key(v, keys)),
        Value::Array(items) => items.iter().any(|v| has_key(v, keys)),
        Value::Null | Value::Bool(_) | Value::Number(_) | Value::String(_) => false,
    }
}

/// A value that Go's `fmt.Sprint` writes as `0`: the number zero or the text `0`.
#[derive(Debug, Default, Clone, Copy, PartialEq, Eq)]
pub(super) struct Zero(pub(super) bool);

#[expect(
    clippy::disallowed_types,
    reason = "a user or group ID may be written as a number or as text"
)]
impl<'de> Deserialize<'de> for Zero {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        Ok(Self(match Value::deserialize(deserializer)? {
            Value::Number(n) => n.as_f64() == Some(0.0),
            Value::String(s) => s == "0",
            Value::Null | Value::Bool(_) | Value::Array(_) | Value::Object(_) => false,
        }))
    }
}

/// Whether a field is set to something other than `null`.
#[derive(Debug, Default, Clone, Copy, PartialEq, Eq)]
pub(super) struct Set(pub(super) bool);

#[expect(
    clippy::disallowed_types,
    reason = "the lint asks only whether a field of any shape is set"
)]
impl<'de> Deserialize<'de> for Set {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        Ok(Self(!Value::deserialize(deserializer)?.is_null()))
    }
}
