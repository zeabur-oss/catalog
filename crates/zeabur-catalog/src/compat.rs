//! `zeabur-catalog compat`: whether the readers of the latest published release can read a new
//! API. Every new file must pass the published schemas, opened: a reader ignores a field it does
//! not know and handles a value it does not know.

#![expect(
    clippy::disallowed_types,
    clippy::disallowed_methods,
    reason = "opening a schema rewrites JSON Schema keywords, of any shape"
)]

use std::path::Path;

use serde_json::Value;
use zeabur_catalog_format::problem::{CompatProblem, Problem};

use crate::api::{Schemas, check, check_as};

/// The problems of the API in `api` against the published API in `published` and its own
/// schemas.
pub(crate) fn compat(published: &Path, api: &Path) -> Result<Vec<Problem>, String> {
    let open = Schemas::read(&published.join("schema"))?.map(opened);
    let mut problems = check_as(api, &open, |p| CompatProblem::PublishedSchema(p).into())?;
    problems.extend(check(api, &Schemas::read(&api.join("schema"))?)?);
    Ok(problems)
}

/// `schema` without `"additionalProperties": false`, `enum`, and a `oneOf` whose every branch is a
/// `const`, which is how the generated schemas write an enum. An `if` or a `not` stays as
/// written: opened, it would match other data, so `then` would apply to every value, or `not` to
/// none.
fn opened(mut schema: Value) -> Value {
    let Some(object) = schema.as_object_mut() else {
        return schema;
    };
    if object.get("additionalProperties") == Some(&Value::Bool(false)) {
        object.remove("additionalProperties");
    }
    object.remove("enum");
    if object
        .get("oneOf")
        .and_then(Value::as_array)
        .is_some_and(|branches| {
            !branches.is_empty() && branches.iter().all(|b| b.get("const").is_some())
        })
    {
        object.remove("oneOf");
    }
    for (key, value) in object.iter_mut() {
        match key.as_str() {
            "properties" | "patternProperties" | "$defs" => {
                if let Some(named) = value.as_object_mut() {
                    for schema in named.values_mut() {
                        *schema = opened(schema.take());
                    }
                }
            }
            "allOf" | "anyOf" | "oneOf" | "prefixItems" => {
                if let Some(list) = value.as_array_mut() {
                    for schema in list {
                        *schema = opened(schema.take());
                    }
                }
            }
            "items" | "additionalProperties" | "then" | "else" | "contains" | "propertyNames" => {
                *value = opened(value.take());
            }
            _ => {}
        }
    }
    schema
}

#[cfg(test)]
mod tests {
    use std::fs;

    use serde_json::json;
    use zeabur_catalog_format::problem::ProblemKind;

    use super::*;
    use crate::api::tests::Api;

    #[test]
    fn opens_fields_and_values_but_not_conditions() {
        // A check of a workload kind names no condition; a property may be called "enum".
        let schema = json!({"items": {
            "additionalProperties": false,
            "properties": {"enum": {"type": "string"}, "kind": {"enum": ["Deployment", "Node"]},
                           "profile": {"oneOf": [{"const": "a"}, {"const": "b"}]}},
            "if": {"properties": {"kind": {"enum": ["Deployment"]}}},
            "then": {"not": {"required": ["condition"]}, "additionalProperties": false}
        }});
        assert_eq!(
            opened(schema),
            json!({"items": {
                "properties": {"enum": {"type": "string"}, "kind": {}, "profile": {}},
                "if": {"properties": {"kind": {"enum": ["Deployment"]}}},
                "then": {"not": {"required": ["condition"]}}
            }})
        );
    }

    #[test]
    fn checks_the_data_against_the_published_schemas() {
        let api = Api::new("compat");
        let published = api.0.join("published");
        fs::create_dir_all(published.join("schema")).unwrap();
        for entry in fs::read_dir(api.dir().join("schema")).unwrap() {
            let entry = entry.unwrap();
            fs::copy(
                entry.path(),
                published.join("schema").join(entry.file_name()),
            )
            .unwrap();
        }
        assert_eq!(compat(&published, &api.dir()).unwrap(), []);

        // A reader ignores a field it does not know and handles a value it does not know.
        let index = api.dir().join("catalog.json");
        let text = fs::read_to_string(&index).unwrap();
        let as_published = Schemas::read(&published.join("schema")).unwrap();
        for (change, pointer, value) in [
            ("a new field", "/added", json!(true)),
            ("a new profile", "/profiles/0", json!("edge")),
        ] {
            let mut doc: Value = serde_json::from_str(&text).unwrap();
            let (parent, key) = pointer.rsplit_once('/').unwrap();
            match doc.pointer_mut(parent).unwrap() {
                Value::Object(object) => drop(object.insert(key.to_owned(), value)),
                Value::Array(list) => list.insert(0, value),
                Value::Null | Value::Bool(_) | Value::Number(_) | Value::String(_) => {
                    unreachable!()
                }
            }
            fs::write(&index, doc.to_string()).unwrap();
            assert_ne!(check(&api.dir(), &as_published).unwrap(), [], "{change}");
            let open = as_published.clone().map(opened);
            assert_eq!(check(&api.dir(), &open).unwrap(), [], "{change}");
        }
        fs::write(&index, &text).unwrap();

        // A published schema that needs a field the new data lacks breaks its readers.
        let schema = published.join("schema/catalog.schema.json");
        let mut doc: Value = serde_json::from_str(&fs::read_to_string(&schema).unwrap()).unwrap();
        doc["required"].as_array_mut().unwrap().push(json!("gone"));
        fs::write(&schema, doc.to_string()).unwrap();
        let problems = compat(&published, &api.dir()).unwrap();
        assert!(
            problems.iter().any(|p| p.file == index
                && matches!(
                    p.kind,
                    ProblemKind::Compat(CompatProblem::PublishedSchema(_))
                )),
            "{problems:?}"
        );
    }
}
