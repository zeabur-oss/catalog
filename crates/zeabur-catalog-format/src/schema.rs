//! The JSON Schemas of `schema/v1/` and the TypeScript types of `types/v1/`, generated from the
//! types of this crate.
#![expect(
    clippy::disallowed_types,
    reason = "a JSON Schema is a document of any shape, which the generated schemas are edited as"
)]

use std::path::Path;

use schemars::generate::SchemaSettings;
use schemars::transform::transform_subschemas;
use schemars::{JsonSchema, Schema, SchemaGenerator, json_schema};
use serde_json::{Map, Value, json};
use ts_rs::{Config, TS};

use crate::api::{ComponentFile, Index, TemplateFile};
use crate::component::Component;
use crate::select::SelectCase;
use crate::template::Template;

/// The address of the published schemas. They name each other by relative `$ref`.
pub const BASE: &str = "https://oss-catalog.zeabur.com/v1/schema/";

/// Each schema of `schema/v1/`: its file name and its text.
#[must_use]
pub fn schemas() -> Vec<(&'static str, String)> {
    vec![
        file::<Component>("component.schema.json", "Component"),
        file::<Template>("template.schema.json", "Template"),
        file::<Index>("catalog.schema.json", "catalog.json"),
        file::<ComponentFile>("component-file.schema.json", "components/<id>.json"),
        file::<TemplateFile>("template-file.schema.json", "templates/<id>.json"),
        file::<SelectCase>("select-case.schema.json", "A case of conformance/select/"),
    ]
}

/// Writes the TypeScript types of every file in [`schemas`] to `<dir>/catalog.ts`, replacing it.
///
/// # Errors
///
/// The file cannot be removed, or `ts-rs` cannot write a type.
pub fn write_typescript(dir: &Path) -> Result<(), ts_rs::ExportError> {
    match std::fs::remove_file(dir.join("catalog.ts")) {
        Err(e) if e.kind() != std::io::ErrorKind::NotFound => return Err(e.into()),
        Ok(()) | Err(_) => {}
    }
    let config = Config::new().with_out_dir(dir);
    Component::export_all(&config)?;
    Template::export_all(&config)?;
    Index::export_all(&config)?;
    ComponentFile::export_all(&config)?;
    TemplateFile::export_all(&config)?;
    SelectCase::export_all(&config)
}

/// The patterns that the schemas give strings.
pub(crate) mod pattern {
    /// A component or template id.
    pub(crate) const ID: &str = "^[a-z0-9]+(-[a-z0-9]+)*$";
    /// A release, health check or work name, or a capability.
    pub(crate) const NAME: &str = "^[a-z0-9-]+$";
    /// An input name.
    pub(crate) const INPUT: &str = "^[a-z][A-Za-z0-9]*$";
    /// One line of text.
    pub(crate) const LINE: &str = "^[^\\n]+$";
    /// A BCP 47 tag in its recommended case, such as `zh-TW`.
    pub(crate) const LOCALE: &str = "^([a-z]{2,3}(-[a-z]{3}){0,3}|[a-z]{4,8})(-[A-Z][a-z]{3})?(-([A-Z]{2}|[0-9]{3}))?(-([a-z0-9]{5,8}|[0-9][a-z0-9]{3}))*(-[a-wyz0-9](-[a-z0-9]{2,8})+)*(-x(-[a-z0-9]{1,8})+)?$";
    /// A `LicenseRef-` id of SPDX.
    pub(crate) const LICENSE_REF: &str = "^LicenseRef-[A-Za-z0-9.-]+$";
    /// A `sha256:` digest.
    pub(crate) const DIGEST: &str = "^sha256:[a-f0-9]{64}$";
    /// A manifest URL, which names its version.
    pub(crate) const VERSION_FIELD: &str = "\\{version\\}";
    /// An `https://` URL.
    pub(crate) const HTTPS: &str = "^https://";
    /// An `http://` or `https://` URL.
    pub(crate) const HTTP: &str = "^https?://";
    /// A `host:port`.
    pub(crate) const HOST_PORT: &str = "^[a-z0-9.-]+:[0-9]+$";
    /// A duration in seconds or minutes, such as `60s`.
    pub(crate) const TIMEOUT: &str = "^[1-9][0-9]*(s|m)$";
    /// A condition type.
    pub(crate) const CONDITION_NAME: &str = "^[A-Za-z]+$";
    /// A `kubectl wait --for` value.
    pub(crate) const CONDITION: &str =
        "^(condition=[A-Za-z][A-Za-z0-9]*(=(True|False|Unknown))?|jsonpath=.+)$";
    /// A workload kind, ready when its rollout is complete, with or without a name.
    pub(crate) const ROLLOUT: &str = "^([Dd]aemon[Ss]et|[Dd]eployment|[Ss]tateful[Ss]et)(/|$)";
    /// A kind, with or without `/<name>`.
    pub(crate) const WAIT_RESOURCE: &str = "^[A-Za-z][A-Za-z0-9.]*(/[a-z0-9][a-z0-9.-]*)?$";
    /// A `kind/name`.
    pub(crate) const TEMPLATE_RESOURCE: &str = "^[a-zA-Z0-9.-]+/[a-z0-9][a-z0-9.-]*$";
    /// A service or `CloudNativePG` cluster name.
    pub(crate) const SERVICE: &str = "^[a-z][a-z0-9-]*$";
    /// The Service a check reaches.
    pub(crate) const SERVICE_REF: &str = "^[a-z0-9][a-z0-9-]*$";
    /// A path from `/`.
    pub(crate) const PATH: &str = "^/";
    /// A database name.
    pub(crate) const DATABASE: &str = "^[A-Za-z][A-Za-z0-9_]*$";
    /// A query that starts with `SELECT`.
    pub(crate) const SELECT: &str = "^[Ss][Ee][Ll][Ee][Cc][Tt] ";
    /// A Secret name.
    pub(crate) const SECRET_NAME: &str = "^[a-z0-9][a-z0-9.-]*$";
    /// A Secret key.
    pub(crate) const SECRET_KEY: &str = "^[A-Za-z0-9_.-]+$";
    /// A CPU quantity, such as `50m`.
    pub(crate) const CPU: &str = "^[0-9]+(\\.[0-9]+)?m?$";
    /// A memory quantity, such as `64Mi`.
    pub(crate) const MEMORY: &str = "^[1-9][0-9]*(Ki|Mi|Gi|Ti|K|M|G|T)?$";
    /// A Kubernetes quantity, such as `256Mi` or `100m`.
    pub(crate) const QUANTITY: &str =
        "^[+-]?([0-9]+(\\.[0-9]*)?|\\.[0-9]+)(([KMGTPE]i)|[numkMGTPE]|([eE][+-]?[0-9]+))?$";
    /// A Git commit.
    pub(crate) const COMMIT: &str = "^[0-9a-f]{40}$";
    /// An `os/architecture`.
    pub(crate) const PLATFORM: &str = "^[a-z0-9]+/[a-z0-9]+$";
}

/// `components/<id>.json` names the schema of its declaration.
pub(crate) fn component_ref(_: &mut SchemaGenerator) -> Schema {
    json_schema!({ "$ref": "component.schema.json" })
}

/// `templates/<id>.json` names the schema of its template.
pub(crate) fn template_ref(_: &mut SchemaGenerator) -> Schema {
    json_schema!({ "$ref": "template.schema.json" })
}

/// A component id.
pub(crate) fn id(_: &mut SchemaGenerator) -> Schema {
    json_schema!({ "type": "string", "pattern": pattern::ID })
}

/// A capability.
pub(crate) fn name(_: &mut SchemaGenerator) -> Schema {
    json_schema!({ "type": "string", "pattern": pattern::NAME })
}

/// Holds the files to what the catalog's check reads: an object has no field its type does not
/// name, and a field that may be left out is not `null`.
fn closed(schema: &mut Schema) {
    if let Some(object) = schema.as_object_mut() {
        if object.get("type") == Some(&json!("object"))
            && object.contains_key("properties")
            && !object.contains_key("additionalProperties")
        {
            object.insert("additionalProperties".to_owned(), false.into());
        }
        if let Some(Value::Array(types)) = object.get_mut("type") {
            types.retain(|t| t != "null");
            if let [only] = types.as_slice() {
                let only = only.clone();
                object.insert("type".to_owned(), only);
            }
        }
        if let Some(Value::Array(any)) = object.get("anyOf")
            && let [some, none] = any.as_slice()
            && none == &json!({ "type": "null" })
            && let Value::Object(some) = some.clone()
        {
            object.remove("anyOf");
            object.extend(some);
        }
    }
    transform_subschemas(&mut closed, schema);
}

/// Writes the fields of each variant struct of an internally tagged enum into the variant's
/// schema, next to its tag, so that `additionalProperties` sees both. A struct no other schema
/// names is then dropped from `$defs`.
fn inline_variants(root: &mut Schema) {
    let Some(defs) = root.get("$defs").and_then(Value::as_object).cloned() else {
        return;
    };
    let mut inlined = Vec::new();
    for value in root.as_object_mut().into_iter().flat_map(Map::values_mut) {
        inline(value, &defs, &mut inlined);
    }
    let text = root.as_value().to_string();
    if let Some(Value::Object(defs)) = root.get_mut("$defs") {
        for name in inlined {
            if !text.contains(&format!("\"#/$defs/{name}\"")) {
                defs.remove(&name);
            }
        }
    }
}

fn inline(value: &mut Value, defs: &Map<String, Value>, inlined: &mut Vec<String>) {
    match value {
        Value::Object(object) => {
            if object.contains_key("properties")
                && let Some(Value::String(reference)) = object.get("$ref")
                && let Some(name) = reference.strip_prefix("#/$defs/")
                && let Some(Value::Object(variant)) = defs.get(name)
            {
                let name = name.to_owned();
                object.remove("$ref");
                for key in ["properties", "required"] {
                    match (object.get_mut(key), variant.get(key)) {
                        (Some(Value::Object(ours)), Some(Value::Object(theirs))) => {
                            ours.extend(theirs.clone());
                        }
                        (Some(Value::Array(ours)), Some(Value::Array(theirs))) => {
                            ours.extend(theirs.iter().cloned());
                        }
                        (None, Some(theirs)) => {
                            object.insert(key.to_owned(), theirs.clone());
                        }
                        _ => {}
                    }
                }
                inlined.push(name);
            }
            for v in object.values_mut() {
                inline(v, defs, inlined);
            }
        }
        Value::Array(items) => {
            for v in items {
                inline(v, defs, inlined);
            }
        }
        Value::Null | Value::Bool(_) | Value::Number(_) | Value::String(_) => {}
    }
}

fn file<T: JsonSchema>(name: &'static str, title: &str) -> (&'static str, String) {
    let settings = SchemaSettings::draft2020_12();
    let mut schema = settings
        .with_transform(closed)
        .into_generator()
        .into_root_schema_for::<T>();
    inline_variants(&mut schema);
    schema.insert("$id".to_owned(), format!("{BASE}{name}").into());
    schema.insert("title".to_owned(), title.into());
    (name, format!("{:#}\n", schema.as_value()))
}
