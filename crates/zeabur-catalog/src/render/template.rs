//! The templates of `zeabur-catalog render`: each template rendered with its e2e values into
//! `out/<version>/templates/<id>.yaml`, checked with the lint and then with kubeconform against
//! the CRDs of only the components its e2e run installs.

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use serde::Deserialize;
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
use serde_json::Value;
use zeabur_catalog_engine::template;
use zeabur_catalog_format::common::{Profile, Reference};
use zeabur_catalog_format::problem::{FieldPath, LintProblem, LintRule, Problem, Problems};
use zeabur_catalog_format::{Catalog, TemplateDir, order};

use crate::lint::{self, Subject};

/// The component an e2e run installs for a template whose `e2e.yaml` sets `s3`.
const S3_COMPONENT: &str = "seaweedfs-operator";

/// The cert-manager `ClusterIssuer` of an e2e run.
const CLUSTER_ISSUER: &str = "ci-selfsigned";

/// `config/e2e.yaml`: the input values every template gets when it declares the input.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct SharedValues {
    #[serde(default)]
    template_inputs: BTreeMap<String, String>,
}

/// `templates/<id>/e2e.yaml`: whether the run starts an S3 store, and the template's own input
/// values.
#[derive(Debug, Default, Deserialize)]
struct E2eValues {
    #[serde(default)]
    s3: bool,
    #[serde(default)]
    inputs: BTreeMap<String, String>,
}

/// The input values of the e2e run of the template `id`, and whether the run starts an S3 store.
/// They are the values of `config/e2e.yaml` that the template declares, overlaid by its own
/// `e2e.yaml`, with `{{ .ID }}`, `{{ .S3.Endpoint }}` and `{{ .S3.Bucket }}` expanded against a
/// placeholder store.
pub(super) fn e2e_inputs(
    root: &Path,
    id: &str,
    dir: &TemplateDir,
) -> Result<(BTreeMap<String, String>, bool), String> {
    let read = |path: PathBuf| match fs::read_to_string(&path) {
        Ok(text) => Ok(Some((path, text))),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(e) => Err(format!("{}: {e}", path.display())),
    };
    let shared: SharedValues = match read(root.join("config/e2e.yaml"))? {
        Some((path, text)) => {
            serde_saphyr::from_str(&text).map_err(|e| format!("{}: {e}", path.display()))?
        }
        None => SharedValues::default(),
    };
    let own: E2eValues = match read(root.join("templates").join(id).join("e2e.yaml"))? {
        Some((path, text)) => {
            serde_saphyr::from_str(&text).map_err(|e| format!("{}: {e}", path.display()))?
        }
        None => E2eValues::default(),
    };
    let mut inputs: BTreeMap<String, String> = dir
        .template
        .inputs
        .iter()
        .filter_map(|i| Some((i.name.clone(), shared.template_inputs.get(&i.name)?.clone())))
        .collect();
    inputs.extend(own.inputs);
    for (name, value) in &mut inputs {
        let mut expanded = String::new();
        let mut rest = value.as_str();
        while let Some(start) = rest.find("{{") {
            let (before, after) = rest.split_at(start);
            let end = after
                .find("}}")
                .ok_or_else(|| format!("input {name}: {{{{ is not closed"))?;
            let field = after.get(2..end).unwrap_or_default().trim();
            expanded.push_str(before);
            expanded.push_str(match field {
                ".ID" => id,
                ".S3.Endpoint" => "https://s3.example.test",
                ".S3.Bucket" => "e2e",
                _ => return Err(format!("input {name}: unknown field {{{{ {field} }}}}")),
            });
            rest = after.get(end + 2..).unwrap_or_default();
        }
        expanded.push_str(rest);
        *value = expanded;
    }
    Ok((inputs, own.s3))
}

/// The components the e2e run of the template installs on single-node, in no order.
pub(super) fn installs(
    catalog: &Catalog,
    id: &str,
    dir: &TemplateDir,
    s3: bool,
) -> Result<BTreeSet<String>, String> {
    let store = Reference::Component(S3_COMPONENT.to_owned());
    let mut found = BTreeSet::new();
    for reference in dir.template.requires.iter().chain(s3.then_some(&store)) {
        let provider = order::provider(&catalog.components, reference, Profile::SingleNode, id)
            .map_err(|e| e.to_string())?;
        let order = order::install_order(&catalog.components, provider, Profile::SingleNode)
            .map_err(|e| e.to_string())?;
        found.extend(order.into_iter().flatten());
    }
    Ok(found)
}

/// Renders the template `id`, writes it for each of `versions`, lints it, and when the lint
/// finds nothing, checks it with kubeconform against the CRDs that `components`, rendered under
/// `out/<last version>/single-node`, install. It returns the problems.
pub(super) fn check(
    root: &Path,
    catalog: &Catalog,
    id: &str,
    dir: &TemplateDir,
    versions: &[String],
    out: &Path,
) -> Result<Vec<Problem>, String> {
    let mut found = Problems::new(&Path::new("templates").join(id).join("manifests.yaml"));
    let (inputs, s3) = match e2e_inputs(root, id, dir) {
        Ok(values) => values,
        Err(message) => {
            found.push(FieldPath::root(), whole(LintRule::E2eInputs { message }));
            return Ok(found.into_vec());
        }
    };
    let namespace = format!("templates-e2e-{id}");
    let domain = format!("{id}.example.test");
    let options = template::Options {
        namespace: &namespace,
        domain: Some(&domain),
        cluster_issuer: Some(CLUSTER_ISSUER),
        inputs: &inputs,
    };
    let docs = match template::render(id, dir, &options) {
        Ok(docs) => docs,
        Err(e) => {
            let message = e.to_string();
            found.push(FieldPath::root(), whole(LintRule::NotRendered { message }));
            return Ok(found.into_vec());
        }
    };
    let text = yaml(&docs)?;
    let mut files = Vec::new();
    for version in versions {
        let file = out
            .join(version)
            .join("templates")
            .join(format!("{id}.yaml"));
        if let Some(parent) = file.parent() {
            fs::create_dir_all(parent).map_err(|e| format!("{}: {e}", parent.display()))?;
        }
        fs::write(&file, &text).map_err(|e| format!("{}: {e}", file.display()))?;
        files.push((version, file));
    }
    crate::output::info(&format!("rendered template {id}"));
    let subject = Subject {
        id,
        template: &dir.template,
        namespace: &namespace,
    };
    let problems = lint::lint(&subject, &docs);
    if !problems.is_empty() {
        for problem in problems {
            found.push(FieldPath::root(), problem);
        }
        return Ok(found.into_vec());
    }
    let components = match installs(catalog, id, dir, s3) {
        Ok(components) => components,
        Err(message) => {
            found.push(FieldPath::root(), whole(LintRule::E2eInputs { message }));
            return Ok(found.into_vec());
        }
    };
    let schemas = PathBuf::from(format!("{}-template-schemas", out.display())).join(id);
    if let Some(last) = versions.last() {
        let rendered = out.join(last).join(Profile::SingleNode.as_str());
        order_schemas(&rendered, &components, &schemas)?;
    }
    for (version, file) in files {
        let message = kubeconform(version, &schemas, &file)?;
        if let Some(message) = message {
            let version = version.clone();
            found.push(
                FieldPath::root(),
                whole(LintRule::Kubeconform { version, message }),
            );
        }
    }
    Ok(found.into_vec())
}

/// Writes the schemas of the CRDs that `components` render under `rendered`, one directory
/// each, to `schemas`. A component that did not render adds none.
fn order_schemas(
    rendered: &Path,
    components: &BTreeSet<String>,
    schemas: &Path,
) -> Result<(), String> {
    for component in components {
        let Ok(entries) = fs::read_dir(rendered.join(component)) else {
            continue;
        };
        let mut paths: Vec<PathBuf> = entries.filter_map(|e| Some(e.ok()?.path())).collect();
        paths.sort();
        for path in paths {
            let objects = fs::read(&path).map_err(|e| format!("{}: {e}", path.display()))?;
            super::crd_schemas(&objects, schemas)
                .map_err(|e| format!("{}: {e}", path.display()))?;
        }
    }
    Ok(())
}

/// `rule`, broken by the template as a whole.
const fn whole(rule: LintRule) -> LintProblem {
    LintProblem {
        at: String::new(),
        rule,
    }
}

/// Checks `file` with kubeconform for `version`, with the CRD schemas under `schemas`. It
/// returns what kubeconform says when the check fails.
fn kubeconform(version: &str, schemas: &Path, file: &Path) -> Result<Option<String>, String> {
    let output = Command::new("kubeconform")
        .args(["-kubernetes-version", version, "-strict"])
        .args(["-schema-location", "default", "-schema-location"])
        .arg(format!(
            "{}/{{{{.Group}}}}/{{{{.ResourceKind}}}}_{{{{.ResourceAPIVersion}}}}.json",
            schemas.display()
        ))
        .arg(file)
        .output()
        .map_err(|e| format!("kubeconform: {e}"))?;
    if output.status.success() {
        return Ok(None);
    }
    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);
    Ok(Some(format!("{}{}", stdout.trim(), stderr.trim())))
}

/// The objects as a YAML stream, one JSON document each.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
fn yaml(docs: &[Value]) -> Result<String, String> {
    let mut text = String::new();
    for doc in docs {
        text.push_str("---\n");
        text.push_str(&serde_json::to_string(doc).map_err(|e| e.to_string())?);
        text.push('\n');
    }
    Ok(text)
}

#[cfg(test)]
mod tests {
    use std::fs;

    use zeabur_catalog_format::Catalog;

    fn root() -> std::path::PathBuf {
        std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../..")
    }

    #[test]
    fn e2e_inputs_expand_the_s3_store_and_the_id() {
        let catalog = Catalog::read(&root()).unwrap();
        let dir = &catalog.templates["dify"];
        let (inputs, s3) = super::e2e_inputs(&root(), "dify", dir).unwrap();
        assert!(s3);
        assert_eq!(inputs["storageEndpoint"], "https://s3.example.test");
        assert_eq!(inputs["storageBucket"], "e2e");
        let components = super::installs(&catalog, "dify", dir, s3).unwrap();
        assert!(components.contains(super::S3_COMPONENT));
    }

    /// A template is checked against the CRDs of what its e2e run installs only, so a custom
    /// resource of a component it does not require fails kubeconform.
    #[test]
    fn a_template_sees_only_the_crds_of_what_it_installs() {
        let dir = std::path::Path::new(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../out/test/template-schemas"
        ));
        for group in ["a", "b"] {
            let crd = format!(
                "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nspec:\n  \
                 group: {group}.example.com\n  names: {{kind: Thing}}\n  versions:\n    - name: \
                 v1\n      schema:\n        openAPIV3Schema: {{type: object, properties: \
                 {{spec: {{type: object}}}}}}\n"
            );
            fs::create_dir_all(dir.join("rendered").join(group)).unwrap();
            fs::write(dir.join("rendered").join(group).join("crds.yaml"), crd).unwrap();
        }
        let components = std::iter::once("a".to_owned()).collect();
        let schemas = dir.join("schemas");
        let result = super::order_schemas(&dir.join("rendered"), &components, &schemas);
        let a = schemas.join("a.example.com/thing_v1.json").exists();
        let b = schemas.join("b.example.com").exists();
        fs::remove_dir_all(&dir).unwrap();
        result.unwrap();
        assert!(a && !b, "a: {a}, b: {b}");
    }

    #[test]
    fn e2e_inputs_refuse_an_unknown_field() {
        let dir = std::path::Path::new(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../out/test/template-e2e"
        ));
        fs::create_dir_all(dir.join("templates/app")).unwrap();
        fs::write(
            dir.join("templates/app/e2e.yaml"),
            "inputs: {a: '{{ .ID }}-x', b: '{{ .S3.SecretKey }}'}\n",
        )
        .unwrap();
        let catalog = Catalog::read(&root()).unwrap();
        let template = catalog.templates.values().next().unwrap();
        let result = super::e2e_inputs(&dir, "app", template);
        fs::remove_dir_all(&dir).unwrap();
        assert!(result.unwrap_err().contains(".S3.SecretKey"));
    }
}
