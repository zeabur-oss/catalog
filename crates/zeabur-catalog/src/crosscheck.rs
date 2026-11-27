//! `zeabur-catalog crosscheck`: `crosscheck.json`, every template rendered with the values of its
//! e2e run. A client renders each template with the same options and compares the objects.

use std::collections::BTreeMap;
use std::fs;
use std::path::Path;

use serde::Serialize;
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
use serde_json::Value;
use zeabur_catalog_engine::template;
use zeabur_catalog_format::Catalog;

use crate::render::template::{CLUSTER_ISSUER, e2e};

/// The render of one template.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct Entry {
    namespace: String,
    domain: String,
    cluster_issuer: &'static str,
    inputs: BTreeMap<String, String>,
    #[expect(
        clippy::disallowed_types,
        reason = "a template renders objects of any kind, custom resources included"
    )]
    objects: Vec<Value>,
}

/// Writes `crosscheck.json` for the templates under `root` to `out`.
pub(crate) fn crosscheck(root: &Path, out: &Path) -> Result<(), String> {
    let catalog = Catalog::read(root).map_err(|e| e.to_string())?;
    let mut all = BTreeMap::new();
    for (id, dir) in &catalog.templates {
        let run = e2e(root, id, dir).map_err(|e| format!("templates/{id}: {e}"))?;
        let objects = template::render(id, dir, &run.options())
            .map_err(|e| format!("templates/{id}: {e}"))?;
        let entry = Entry {
            namespace: run.namespace,
            domain: run.domain,
            cluster_issuer: CLUSTER_ISSUER,
            inputs: run.inputs,
            objects,
        };
        all.insert(id.as_str(), entry);
    }
    let mut text = serde_json::to_string_pretty(&all).map_err(|e| e.to_string())?;
    text.push('\n');
    fs::write(out, text).map_err(|e| format!("{}: {e}", out.display()))
}

#[cfg(test)]
mod tests {
    /// Every template renders, and an entry holds the options it rendered with.
    #[test]
    fn every_template_renders_with_its_e2e_options() {
        let root = std::path::Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../.."));
        let out = std::path::Path::new(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../out/test/crosscheck.json"
        ));
        std::fs::create_dir_all(out.parent().unwrap()).unwrap();
        super::crosscheck(root, out).unwrap();
        let text = std::fs::read_to_string(out).unwrap();
        std::fs::remove_file(out).unwrap();
        #[expect(clippy::disallowed_types, reason = "the test reads the file back")]
        let all: serde_json::Value = serde_json::from_str(&text).unwrap();
        let ghost = &all["ghost"];
        assert_eq!(ghost["namespace"], "templates-e2e-ghost");
        assert_eq!(ghost["domain"], "ghost.example.test");
        assert_eq!(ghost["clusterIssuer"], "ci-selfsigned");
        assert!(ghost["objects"].as_array().is_some_and(|o| o.len() > 1));
    }
}
