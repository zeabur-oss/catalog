//! `zeabur-catalog check`: the problems of a catalog checkout that CI finds without a cluster.
//!
//! Each component and template file must pass its schema in `schema/v1/`. The components must
//! then fit together: references, install order, contributions, inputs and what they provision.
//! Each template's manifests must parse, with no input name or Secret key twice.

use std::path::Path;
use std::{fmt, fs};

use regex::Regex;
use serde::Deserialize;
use serde_json::Value;
use zeabur_catalog_engine::template;
use zeabur_catalog_format::common::{Generate, Mode, Profile};
use zeabur_catalog_format::component::{Release, Resources};
use zeabur_catalog_format::{Catalog, Component, order};

/// One problem of one file.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Problem {
    /// The file, or `components` for a problem of the components together.
    pub(crate) path: String,
    /// What is wrong.
    pub(crate) message: String,
}

impl fmt::Display for Problem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.path, self.message)
    }
}

/// The problems of the checkout at `root`. Schema problems come alone: the other rules read the
/// files as their types.
///
/// # Errors
///
/// A file or a schema cannot be read.
pub(crate) fn check(root: &Path) -> Result<Vec<Problem>, String> {
    let problems = schema_problems(root)?;
    if !problems.is_empty() {
        return Ok(problems);
    }
    let catalog = Catalog::read(root).map_err(|e| e.to_string())?;
    let mut problems = component_problems(&catalog);
    for (id, dir) in &catalog.templates {
        if let Err(e) = template::check(id, dir) {
            problems.push(problem(
                &format!("templates/{id}/template.yaml"),
                e.to_string(),
            ));
        }
    }
    Ok(problems)
}

/// The files of each kind and the schema they pass.
const SCHEMAS: [(&str, &str, &str); 2] = [
    ("components", "component.yaml", "component.schema.json"),
    ("templates", "template.yaml", "template.schema.json"),
];

fn schema_problems(root: &Path) -> Result<Vec<Problem>, String> {
    let mut problems = Vec::new();
    for (dir, file, schema) in SCHEMAS {
        let schema_path = root.join("schema/v1").join(schema);
        let schema: Value = serde_json::from_str(&read(&schema_path)?)
            .map_err(|e| format!("{}: {e}", schema_path.display()))?;
        let validator = jsonschema::validator_for(&schema)
            .map_err(|e| format!("{}: {e}", schema_path.display()))?;
        let mut ids: Vec<String> = fs::read_dir(root.join(dir))
            .map_err(|e| format!("{dir}: {e}"))?
            .map(|entry| entry.map(|e| e.file_name().to_string_lossy().into_owned()))
            .collect::<Result<_, _>>()
            .map_err(|e| format!("{dir}: {e}"))?;
        ids.sort_unstable();
        for id in ids {
            let path = format!("{dir}/{id}/{file}");
            let text = read(&root.join(&path))?;
            let instance: Value = match serde_saphyr::from_str(&text) {
                Ok(instance) => instance,
                Err(e) => {
                    problems.push(problem(&path, e.to_string()));
                    continue;
                }
            };
            for error in validator.iter_errors(&instance) {
                problems.push(problem(
                    &path,
                    format!("{}: {error}", error.instance_path()),
                ));
            }
        }
    }
    Ok(problems)
}

fn read(path: &Path) -> Result<String, String> {
    fs::read_to_string(path).map_err(|e| format!("{}: {e}", path.display()))
}

fn problem(path: &str, message: String) -> Problem {
    Problem {
        path: path.to_owned(),
        message,
    }
}

/// The problems of the components, each on its own and all together.
pub(crate) fn component_problems(catalog: &Catalog) -> Vec<Problem> {
    let mut problems = Vec::new();
    for (id, component) in &catalog.components {
        let path = format!("components/{id}/component.yaml");
        problems.extend(
            problems_of(catalog, id, component)
                .into_iter()
                .map(|message| problem(&path, message)),
        );
    }
    for profile in Profile::ALL {
        let order_problem = catalog
            .components
            .iter()
            .filter(|(_, c)| c.mode(profile) != Mode::Unsupported)
            .find_map(|(id, _)| order::install_order(&catalog.components, id, profile).err());
        if let Some(e) = order_problem {
            problems.push(problem("components", format!("{}: {e}", profile.as_str())));
        }
    }
    problems
}

fn problems_of(catalog: &Catalog, id: &str, component: &Component) -> Vec<String> {
    let mut out = Vec::new();
    if !is_id(id) {
        out.push(
            "the directory name must be lowercase letters, digits and single hyphens".to_owned(),
        );
    }
    // An installer reads a name in enable or disable as a component id first, so that id must
    // not hide another component's capability.
    if let Some(capability) = component.capability()
        && catalog
            .components
            .get(capability)
            .is_some_and(|other| other.capability() != Some(capability))
    {
        out.push(format!(
            "provides.capability: {capability} is the ID of another component"
        ));
    }
    for release in &component.releases {
        if let Release::Manifest(r) = release
            && r.url_digests.len() != r.urls.len()
        {
            out.push(format!(
                "{}: urlDigests needs one digest for each of the {} urls",
                r.name,
                r.urls.len()
            ));
        }
    }
    for input in &component.inputs {
        if input.path.is_some() && component.input_release(input).is_none() {
            out.push(format!(
                "input {}: no chart release for its path",
                input.name
            ));
        }
        if input.required
            && input.default.is_none()
            && input.test_value.is_none()
            && input.generate.is_none()
        {
            out.push(format!(
                "input {}: required, so set default, testValue or generate for CI to install it",
                input.name
            ));
        }
    }
    out.extend(provision_problems(component));
    if let Some(resources) = &component.resources {
        out.extend(work_problems(resources));
    }
    out.extend(contribution_problems(catalog, component));
    out
}

/// Whether `id` names a component or template: lowercase words joined by single hyphens.
fn is_id(id: &str) -> bool {
    id.split('-').all(|word| {
        !word.is_empty()
            && word
                .bytes()
                .all(|b| b.is_ascii_lowercase() || b.is_ascii_digit())
    })
}

/// An object of `networkPolicies`, as far as the check reads it.
#[derive(Deserialize)]
struct Object {
    kind: String,
    metadata: ObjectMeta,
}

#[derive(Deserialize)]
struct ObjectMeta {
    name: String,
    #[serde(default)]
    namespace: Option<String>,
}

/// What `provides`, `internalCRDs`, generated inputs and `networkPolicies` name, against the
/// component's own CRDs, inputs and namespaces.
fn provision_problems(component: &Component) -> Vec<String> {
    let mut out = Vec::new();
    let owns = |namespace: &str| {
        component
            .releases
            .iter()
            .any(|r| r.namespace() == Some(namespace))
    };
    if let Some(registry) = component
        .provides
        .as_ref()
        .and_then(|p| p.registry.as_ref())
        && !owns(&registry.ca.namespace)
    {
        out.push(format!(
            "provides.registry.ca: {} is none of its namespaces",
            registry.ca.namespace
        ));
    }
    for crd in &component.internal_crds {
        if !component.detection.crds.contains(crd) {
            out.push(format!("internalCRDs: {crd} is not in detection.crds"));
        }
    }
    for input in &component.inputs {
        if input.generate == Some(Generate::ClusterIp) && input.path.is_none() {
            out.push(format!(
                "input {}: generate clusterIP needs a path",
                input.name
            ));
        }
    }
    let policies = component.network_policies.as_deref().unwrap_or_default();
    match serde_saphyr::from_multiple::<Option<Object>>(policies) {
        Err(e) => out.push(format!("networkPolicies: {e}")),
        Ok(objects) => {
            for object in objects.into_iter().flatten() {
                if object.kind != "NetworkPolicy"
                    || !object.metadata.namespace.as_deref().is_some_and(owns)
                {
                    out.push(format!(
                        "networkPolicies: {} {} must be a NetworkPolicy in one of its namespaces",
                        object.kind, object.metadata.name
                    ));
                }
            }
        }
    }
    out
}

fn work_problems(resources: &Resources) -> Vec<String> {
    let mut out = Vec::new();
    let mut seen = Vec::new();
    for work in &resources.work {
        if seen.contains(&work.name.as_str()) {
            out.push(format!("resources.work: {} is listed twice", work.name));
        }
        seen.push(&work.name);
        for (field, quantities) in [
            ("requests", &work.requests),
            ("limits", &work.limits),
            ("use", &work.uses),
        ] {
            let values = quantities.iter().flat_map(|q| {
                [("cpu", &q.cpu), ("memory", &q.memory)]
                    .into_iter()
                    .filter_map(|(resource, value)| Some((resource, value.as_deref()?)))
            });
            for (resource, value) in values {
                if let Err(e) = above_zero(value) {
                    out.push(format!(
                        "resources.work {}: {field}: {resource}: {e}",
                        work.name
                    ));
                }
            }
        }
    }
    out
}

/// A Kubernetes quantity [quantity], such as `256Mi` or `100m`.
///
/// [quantity]: https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/
const QUANTITY: &str =
    r"\A[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)(([KMGTPE]i)|[numkMGTPE]|([eE][+-]?[0-9]+))?\z";

/// Whether `value` is a quantity above zero.
fn above_zero(value: &str) -> Result<(), String> {
    let quantity = Regex::new(QUANTITY).map_err(|e| e.to_string())?;
    let Some(number) = quantity.captures(value).and_then(|c| c.get(1)) else {
        return Err(format!("{value} is not a quantity"));
    };
    if value.starts_with('-') || !number.as_str().bytes().any(|b| matches!(b, b'1'..=b'9')) {
        return Err(format!("{value} is not above zero"));
    }
    Ok(())
}

fn contribution_problems(catalog: &Catalog, component: &Component) -> Vec<String> {
    let mut out = Vec::new();
    // An installer finds the contributors it installed by their Helm releases.
    if !component.contributes.is_empty() && !component.releases.iter().any(|r| r.helm().is_some()) {
        out.push("contributes: needs a helm release of its own".to_owned());
    }
    for contribution in &component.contributes {
        let target = &contribution.target;
        match catalog.components.get(&target.component) {
            None => out.push(format!("contributes: unknown target {}", target.component)),
            Some(other) if other.release(&target.release).is_none() => out.push(format!(
                "contributes: {} has no release {}",
                target.component, target.release
            )),
            Some(_) => {}
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn root() -> &'static Path {
        Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../.."))
    }

    fn catalog() -> Catalog {
        Catalog::read(root()).unwrap()
    }

    #[test]
    fn the_repository_has_no_problems() {
        assert_eq!(check(root()).unwrap(), Vec::new());
    }

    /// The problems of the repository with `edit` made to one component.
    fn problems(id: &str, edit: impl FnOnce(&mut Component)) -> Vec<String> {
        let mut catalog = catalog();
        edit(catalog.components.get_mut(id).unwrap());
        component_problems(&catalog)
            .into_iter()
            .map(|p| p.to_string())
            .collect()
    }

    #[test]
    fn a_capability_must_not_be_the_id_of_another_component() {
        let got = problems("traefik", |c| {
            c.provides.as_mut().unwrap().capability = Some("cert-manager".to_owned());
        });
        assert!(
            got.contains(&"components/traefik/component.yaml: provides.capability: cert-manager is the ID of another component".to_owned()),
            "{got:?}"
        );
    }

    #[test]
    fn a_contributor_needs_a_helm_release() {
        let got = problems("crowdsec", |c| c.releases.retain(|r| r.helm().is_none()));
        assert!(
            got.contains(
                &"components/crowdsec/component.yaml: contributes: needs a helm release of its own"
                    .to_owned()
            ),
            "{got:?}"
        );
    }

    #[test]
    fn a_requirement_must_name_a_component_on_every_profile() {
        let got = problems("traefik", |c| {
            c.requires = vec![zeabur_catalog_format::common::Reference::Component(
                "no-such-component".to_owned(),
            )];
        });
        assert!(
            got.iter()
                .any(|p| p.starts_with("components: single-node: ")
                    && p.contains("no-such-component")),
            "{got:?}"
        );
    }

    #[test]
    fn provisions_only_its_own_crds_inputs_and_namespaces() {
        let ok = "kind: NetworkPolicy\nmetadata: {name: a, namespace: zot}\n";
        let cases: &[(&str, Option<&str>, bool)] = &[
            ("its own namespace", Some(ok), false),
            (
                "a policy in another namespace",
                Some("kind: NetworkPolicy\nmetadata: {name: a, namespace: default}\n"),
                true,
            ),
            (
                "another kind",
                Some("kind: ConfigMap\nmetadata: {name: a, namespace: zot}\n"),
                true,
            ),
            (
                "no namespace",
                Some("kind: NetworkPolicy\nmetadata: {name: a}\n"),
                true,
            ),
        ];
        for (name, policies, bad) in cases {
            let got = problems("zeabur-builder", |c| {
                c.network_policies = policies.map(str::to_owned);
            });
            assert_eq!(!got.is_empty(), *bad, "{name}: {got:?}");
        }
        let got = problems("zeabur-builder", |c| {
            c.provides
                .as_mut()
                .unwrap()
                .registry
                .as_mut()
                .unwrap()
                .ca
                .namespace = "default".to_owned();
        });
        assert!(
            got.iter()
                .any(|p| p.ends_with("provides.registry.ca: default is none of its namespaces")),
            "{got:?}"
        );
        let got = problems("zeabur-builder", |c| {
            c.internal_crds = vec!["other.example.com".to_owned()];
        });
        assert!(
            got.iter()
                .any(|p| p.ends_with("internalCRDs: other.example.com is not in detection.crds")),
            "{got:?}"
        );
    }

    #[test]
    fn quantities_are_above_zero() {
        for ok in ["1", "100m", "0.5", "256Mi", "1e3", "+1"] {
            assert_eq!(above_zero(ok), Ok(()), "{ok}");
        }
        for bad in ["0", "0m", "-1", "0.0", "abc", "1Mb", ""] {
            assert!(above_zero(bad).is_err(), "{bad}");
        }
    }

    #[test]
    fn ids_are_lowercase_words() {
        for ok in ["traefik", "cert-manager", "k8s-1"] {
            assert!(is_id(ok), "{ok}");
        }
        for bad in ["Traefik", "cert--manager", "-a", "a-", "", "a_b"] {
            assert!(!is_id(bad), "{bad}");
        }
    }
}
