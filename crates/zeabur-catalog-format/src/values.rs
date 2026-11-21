//! Helm values of a release: the release's own, its profile's, what other components contribute,
//! and the inputs at their paths.

use serde_json::{Map, Value};

use crate::common::Profile;
use crate::component::{Component, HelmRelease};

/// A values document: a YAML mapping.
pub type Values = Map<String, Value>;

/// Values that do not read, or an input path that does not fit them.
#[derive(Debug, thiserror::Error)]
pub enum ValuesError {
    /// The text is no YAML mapping.
    #[error("values are no YAML mapping: {0}")]
    Parse(String),
    /// An input path runs into a value that is no mapping or list.
    #[error("input {input}: cannot set {key} inside a {found}")]
    Path {
        /// The input.
        input: String,
        /// The key of the path.
        key: String,
        /// What is there.
        found: &'static str,
    },
    /// An input path gives a key, not an index, for a list.
    #[error("input {input}: {key} is a list")]
    NotIndex {
        /// The input.
        input: String,
        /// The key of the list.
        key: String,
    },
}

/// Reads a values document. Empty text is an empty mapping.
///
/// # Errors
///
/// The text is no YAML mapping.
pub fn parse(text: &str) -> Result<Values, ValuesError> {
    if text.trim().is_empty() {
        return Ok(Values::new());
    }
    serde_saphyr::from_str(text).map_err(|e| ValuesError::Parse(e.to_string()))
}

/// Merges `src` into `dst`: a mapping in both merges key by key; anything else in `src` replaces.
pub fn merge(dst: &mut Values, src: Values) {
    for (key, value) in src {
        match (dst.get_mut(&key), value) {
            (Some(Value::Object(d)), Value::Object(s)) => merge(d, s),
            (_, value) => {
                dst.insert(key, value);
            }
        }
    }
}

/// Sets `value` at a dotted `path`, such as `controller.replicas` or `args.0`.
///
/// A missing key becomes a list when the next key is a number and a mapping otherwise; a list
/// grows with empty mappings up to the index.
///
/// # Errors
///
/// The path runs into a scalar, or gives a list a key that is no index. `input` names the input
/// in the error.
pub fn set_path(
    values: &mut Values,
    path: &str,
    value: Value,
    input: &str,
) -> Result<(), ValuesError> {
    let keys: Vec<&str> = path.split('.').collect();
    let mut node = values;
    let mut rest = keys.as_slice();
    loop {
        let Some((key, next)) = rest.split_first() else {
            return Ok(());
        };
        let Some(after) = next.first() else {
            node.insert((*key).to_owned(), value);
            return Ok(());
        };
        let child = node.entry(*key).or_insert_with(|| {
            if after.parse::<usize>().is_ok() {
                Value::Array(Vec::new())
            } else {
                Value::Object(Map::new())
            }
        });
        match child {
            Value::Object(map) => {
                node = map;
                rest = next;
            }
            Value::Array(list) => {
                let Ok(index) = after.parse::<usize>() else {
                    return Err(ValuesError::NotIndex {
                        input: input.to_owned(),
                        key: (*key).to_owned(),
                    });
                };
                while list.len() <= index {
                    list.push(Value::Object(Map::new()));
                }
                let Some(slot) = list.get_mut(index) else {
                    return Ok(());
                };
                let Some(deeper) = next.get(1..).filter(|d| !d.is_empty()) else {
                    *slot = value;
                    return Ok(());
                };
                let Value::Object(map) = slot else {
                    return Err(ValuesError::Path {
                        input: input.to_owned(),
                        key: (*after).to_owned(),
                        found: kind(slot),
                    });
                };
                node = map;
                rest = deeper;
            }
            other @ (Value::Null | Value::Bool(_) | Value::Number(_) | Value::String(_)) => {
                return Err(ValuesError::Path {
                    input: input.to_owned(),
                    key: (*after).to_owned(),
                    found: kind(other),
                });
            }
        }
    }
}

const fn kind(value: &Value) -> &'static str {
    match value {
        Value::Null => "null",
        Value::Bool(_) => "boolean",
        Value::Number(_) => "number",
        Value::String(_) => "string",
        Value::Array(_) => "list",
        Value::Object(_) => "mapping",
    }
}

/// The release's own values with its profile's merged over them.
///
/// # Errors
///
/// Either document is no YAML mapping.
pub fn profile_values(release: &HelmRelease, profile: Profile) -> Result<Values, ValuesError> {
    let mut values = parse(release.values.as_deref().unwrap_or_default())?;
    if let Some(over) = release.profiles.get(profile) {
        merge(&mut values, parse(over)?);
    }
    Ok(values)
}

/// The values a chart release installs with.
///
/// They are its profile values, then `contributed` (what the components installed before it add,
/// [`Contributions::to`](crate::contributions::Contributions::to)), then each input of
/// `component` with a value in `inputs` and a `path` into this release.
///
/// # Errors
///
/// A document is no YAML mapping, or an input path does not fit.
pub fn release_values(
    component: &Component,
    release: &HelmRelease,
    profile: Profile,
    contributed: Values,
    inputs: &Map<String, Value>,
) -> Result<Values, ValuesError> {
    let mut values = profile_values(release, profile)?;
    merge(&mut values, contributed);
    for input in &component.inputs {
        let (Some(path), Some(value)) = (input.path.as_deref(), inputs.get(&input.name)) else {
            continue;
        };
        if component
            .input_release(input)
            .map(crate::component::Release::name)
            == Some(release.name.as_str())
        {
            set_path(&mut values, path, value.clone(), &input.name)?;
        }
    }
    Ok(values)
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;

    fn map(value: Value) -> Values {
        let Value::Object(m) = value else {
            unreachable!()
        };
        m
    }

    #[test]
    fn merge_joins_mappings_and_replaces_the_rest() {
        let mut dst = map(json!({"a": {"x": 1, "y": [1]}, "b": 1}));
        merge(
            &mut dst,
            map(json!({"a": {"y": [2], "z": 3}, "b": {"c": 1}})),
        );
        assert_eq!(
            Value::Object(dst),
            json!({"a": {"x": 1, "y": [2], "z": 3}, "b": {"c": 1}})
        );
    }

    #[test]
    fn set_path_makes_mappings_and_lists() {
        let mut values = Values::new();
        set_path(&mut values, "a.b", json!(1), "i").unwrap();
        set_path(&mut values, "l.1.name", json!("x"), "i").unwrap();
        set_path(&mut values, "s.0", json!(true), "i").unwrap();
        assert_eq!(
            Value::Object(values),
            json!({"a": {"b": 1}, "l": [{}, {"name": "x"}], "s": [true]})
        );
    }

    #[test]
    fn set_path_refuses_to_go_through_a_scalar() {
        let mut values = map(json!({"a": 1, "l": [1]}));
        assert!(matches!(
            set_path(&mut values, "a.b", json!(1), "i"),
            Err(ValuesError::Path { .. })
        ));
        assert!(matches!(
            set_path(&mut values, "l.key", json!(1), "i"),
            Err(ValuesError::NotIndex { .. })
        ));
        assert!(matches!(
            set_path(&mut values, "l.0.x", json!(1), "i"),
            Err(ValuesError::Path { .. })
        ));
    }

    #[test]
    fn parse_takes_a_mapping_only() {
        assert!(parse("").unwrap().is_empty());
        assert_eq!(parse("a: 1").unwrap().get("a"), Some(&json!(1)));
        assert!(parse("- 1").is_err());
    }
}
