//! `zeabur-catalog template render|inputs`: a template of a release's static API, rendered for a
//! namespace or with its generated inputs, to deploy it by hand.

use std::collections::BTreeMap;
use std::fs;
use std::path::Path;

use base64::Engine as _;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use regex::Regex;
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
use serde_json::Value;
use serde_json::json;
use zeabur_catalog_engine::dns::is_dns_label;
use zeabur_catalog_engine::template;
use zeabur_catalog_format::TemplateDir;
use zeabur_catalog_format::api::TemplateFile;
use zeabur_catalog_format::common::{Encoding, Generate};

use crate::check::is_id;

/// What to render a template with.
#[derive(Debug, Default)]
pub(crate) struct Options {
    /// The namespace.
    pub(crate) namespace: String,
    /// The hostname of the services with domains.
    pub(crate) domain: Option<String>,
    /// The cert-manager `ClusterIssuer` of the services with domains.
    pub(crate) cluster_issuer: Option<String>,
    /// A JSON file with an object of input values.
    pub(crate) values: Option<String>,
}

/// The template `id` from `<api>/templates/<id>.json`, where `api` is the `v1` directory of a
/// release's `api-v1.tar.gz`.
pub(crate) fn read(api: &Path, id: &str) -> Result<TemplateDir, String> {
    if !is_id(id) {
        return Err(format!("{id:?} is not a template ID"));
    }
    let path = api.join("templates").join(format!("{id}.json"));
    let text = fs::read(&path).map_err(|e| format!("{}: {e}", path.display()))?;
    let file: TemplateFile =
        serde_json::from_slice(&text).map_err(|e| format!("{}: {e}", path.display()))?;
    if file.id != id {
        return Err(format!(
            "{}: the file is template {}",
            path.display(),
            file.id
        ));
    }
    Ok(TemplateDir {
        template: file.template,
        manifests: file.manifests,
    })
}

/// The objects of the template `id` for `options`, as a YAML stream of JSON documents.
pub(crate) fn render(id: &str, dir: &TemplateDir, options: &Options) -> Result<String, String> {
    let inputs: BTreeMap<String, String> = match &options.values {
        None => BTreeMap::new(),
        Some(path) => {
            let text = fs::read(path).map_err(|e| format!("{path}: {e}"))?;
            serde_json::from_slice(&text)
                .map_err(|e| format!("{path}: want a JSON object of strings: {e}"))?
        }
    };
    let objects = template::render(
        id,
        dir,
        &template::Options {
            namespace: &options.namespace,
            domain: options.domain.as_deref(),
            cluster_issuer: options.cluster_issuer.as_deref(),
            inputs: &inputs,
        },
    )
    .map_err(|e| e.to_string())?;
    let mut text = String::new();
    for object in &objects {
        text.push_str("---\n");
        text.push_str(&serde_json::to_string(object).map_err(|e| e.to_string())?);
        text.push('\n');
    }
    Ok(text)
}

/// A `List` of the Secrets that hold the generated inputs of the template `id` in `namespace`,
/// with values from `random`, which fills its buffer with random bytes.
#[expect(
    clippy::disallowed_types,
    reason = "a List of Secrets as kubectl applies it"
)]
pub(crate) fn inputs(
    id: &str,
    dir: &TemplateDir,
    namespace: &str,
    mut random: impl FnMut(&mut [u8]) -> Result<(), String>,
) -> Result<Value, String> {
    if !is_dns_label(namespace) {
        return Err(format!("namespace {namespace:?} is not a DNS label"));
    }
    template::check(id, dir).map_err(|e| e.to_string())?;
    let mut secrets: BTreeMap<&str, BTreeMap<&str, String>> = BTreeMap::new();
    for input in &dir.template.inputs {
        let (Some(generate), Some(secret)) = (&input.generate, &input.secret) else {
            continue;
        };
        let value = match generate {
            Generate::Random { bytes, encoding } => {
                let mut buf = vec![0; usize::from(*bytes)];
                random(&mut buf)?;
                match encoding {
                    Encoding::Hex => crate::render::hex(&buf),
                    Encoding::Base64url => URL_SAFE_NO_PAD.encode(&buf),
                }
            }
            Generate::ClusterIp | Generate::Unknown => {
                return Err(format!(
                    "input {}: a template generates random values only",
                    input.name
                ));
            }
        };
        let pattern = Regex::new(&format!(r"\A(?:{})\z", input.validation.pattern))
            .map_err(|e| format!("input {}: {e}", input.name))?;
        if !pattern.is_match(&value) {
            return Err(format!(
                "input {}: a generated value does not match its pattern",
                input.name
            ));
        }
        secrets
            .entry(secret.name.as_str())
            .or_default()
            .insert(secret.key.as_str(), value);
    }
    let labels = template::labels(id, &dir.template);
    let items: Vec<Value> = secrets
        .into_iter()
        .map(|(name, data)| {
            json!({
                "apiVersion": "v1",
                "kind": "Secret",
                "metadata": {"name": name, "namespace": namespace, "labels": labels},
                "type": "Opaque",
                "stringData": data,
            })
        })
        .collect();
    Ok(json!({"apiVersion": "v1", "kind": "List", "items": items}))
}

/// Fills `buf` from the operating system's random source.
pub(crate) fn os_random(buf: &mut [u8]) -> Result<(), String> {
    getrandom::fill(buf).map_err(|e| format!("random bytes: {e}"))
}

#[cfg(test)]
mod tests {
    use zeabur_catalog_format::Catalog;

    fn root() -> std::path::PathBuf {
        std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../..")
    }

    /// Each generated input lands in its Secret key, encoded as the input says, and matches its
    /// pattern.
    #[test]
    fn generated_inputs_fill_their_secret_keys() {
        let catalog = Catalog::read(&root()).unwrap();
        let (id, dir) = catalog
            .templates
            .iter()
            .find(|(_, d)| d.template.inputs.iter().any(|i| i.generate.is_some()))
            .unwrap();
        let list = super::inputs(id, dir, "app", |buf| {
            buf.fill(0xAB);
            Ok(())
        })
        .unwrap();
        for input in dir.template.inputs.iter().filter(|i| i.generate.is_some()) {
            let secret = input.secret.as_ref().unwrap();
            let item = list["items"]
                .as_array()
                .unwrap()
                .iter()
                .find(|s| s["metadata"]["name"] == secret.name.as_str())
                .unwrap();
            assert_eq!(item["metadata"]["namespace"], "app");
            assert_eq!(
                item["metadata"]["labels"]["templates.zeabur.com/template"],
                id.as_str()
            );
            let value = item["stringData"][&secret.key].as_str().unwrap();
            assert!(
                value.starts_with("ab") || value.starts_with("q6"),
                "{value}"
            );
        }
    }

    #[test]
    fn inputs_refuse_a_namespace_that_is_not_a_dns_label() {
        let catalog = Catalog::read(&root()).unwrap();
        let (id, dir) = catalog.templates.iter().next().unwrap();
        let err = super::inputs(id, dir, "App", |_| Ok(())).unwrap_err();
        assert!(err.contains("DNS label"), "{err}");
    }

    #[test]
    fn read_refuses_an_id_that_is_not_one() {
        let err = super::read(std::path::Path::new("."), "../x").unwrap_err();
        assert!(err.contains("not a template ID"), "{err}");
    }
}
