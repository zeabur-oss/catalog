//! The generated inputs of a template as Secrets, which the dry run applies before the template.

use std::collections::BTreeMap;

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
use zeabur_catalog_format::common::{Encoding, Generate};

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
}
