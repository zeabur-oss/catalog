//! `zeabur-catalog check`: the problems of a catalog checkout that CI finds without a cluster.
//!
//! Each component and template file must pass its schema in `schema/v1/`. The components must
//! then fit together: references, install order, contributions, inputs and what they provision.
//! Each template must render, and fit the repository: its ID, services, version and requires. Then
//! the repository's policy: licences, languages, READMEs and Kubernetes versions.
#![expect(
    clippy::disallowed_types,
    reason = "a file is checked against its JSON Schema as JSON of any shape"
)]

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use jsonschema::paths::LocationSegment;
use regex::Regex;
use serde::Deserialize;
use serde_json::Value;
use zeabur_catalog_engine::template;
use zeabur_catalog_format::check::StrictProblem;
use zeabur_catalog_format::common::{Generate, Mode, Profile};
use zeabur_catalog_format::component::{Release, Resources};
use zeabur_catalog_format::problem::{
    ComponentProblem, FieldPath, Problem, Problems, QuantityProblem, SchemaProblem, TemplateProblem,
};
use zeabur_catalog_format::{Catalog, Component, order};

use crate::{policy, templates};

/// The problems of the checkout at `root`. Schema problems come alone: the other rules read the
/// files as their types.
///
/// # Errors
///
/// A file or a schema cannot be read.
pub(crate) fn check(root: &Path) -> Result<Vec<Problem>, String> {
    let (problems, files) = schema_problems(root)?;
    if !problems.is_empty() {
        return Ok(problems);
    }
    let catalog = Catalog::read(root).map_err(|e| e.to_string())?;
    let config = policy::Config::read(root)?;
    let mut problems = component_problems(&catalog);
    let null = Value::Null;
    for (id, component) in &catalog.components {
        let raw = files.get(&component_file(id)).unwrap_or(&null);
        let dir = Path::new("components").join(id);
        problems.extend(policy::component_problems(
            &config, root, &dir, component, raw,
        ));
    }
    for (id, dir) in &catalog.templates {
        let path = template_file(id);
        let mut found = Problems::new(&path);
        if let Err(e) = template::check(id, dir) {
            let message = e.to_string();
            found.push(FieldPath::root(), TemplateProblem::Broken { message });
        }
        let text = read(&root.join(&path))?;
        templates::problems(&catalog, id, dir, &text, &mut found);
        problems.extend(found.into_vec());
        let raw = files.get(&path).unwrap_or(&null);
        let template_dir = Path::new("templates").join(id);
        problems.extend(policy::template_problems(
            &config,
            root,
            &template_dir,
            &dir.template,
            raw,
        ));
    }
    Ok(problems)
}

fn template_file(id: &str) -> PathBuf {
    Path::new("templates").join(id).join("template.yaml")
}

fn component_file(id: &str) -> PathBuf {
    Path::new("components").join(id).join("component.yaml")
}

/// The files of each kind and the schema they pass.
const SCHEMAS: [(&str, &str, &str); 2] = [
    ("components", "component.yaml", "component.schema.json"),
    ("templates", "template.yaml", "template.schema.json"),
];

/// The validator of `schema`. It asserts `format`, which Draft 2020-12 only annotates by default.
pub(crate) fn validator(schema: &Value) -> Result<jsonschema::Validator, String> {
    jsonschema::options()
        .should_validate_formats(true)
        .build(schema)
        .map_err(|e| e.to_string())
}

/// The place in a file that a schema error names.
pub(crate) fn field_path(location: &jsonschema::paths::Location) -> FieldPath {
    location
        .into_iter()
        .fold(FieldPath::root(), |at, segment| match segment {
            LocationSegment::Property(key) => at.key(key),
            LocationSegment::Index(index) => at.index(index),
        })
}

/// The schema problems of each file, and each file read as it is written, by path.
fn schema_problems(root: &Path) -> Result<(Vec<Problem>, BTreeMap<PathBuf, Value>), String> {
    let mut problems = Vec::new();
    let mut files = BTreeMap::new();
    for (dir, file, schema) in SCHEMAS {
        let validator = schema_validator(root, schema)?;
        let mut ids: Vec<String> = fs::read_dir(root.join(dir))
            .map_err(|e| format!("{dir}: {e}"))?
            .map(|entry| entry.map(|e| e.file_name().to_string_lossy().into_owned()))
            .collect::<Result<_, _>>()
            .map_err(|e| format!("{dir}: {e}"))?;
        ids.sort_unstable();
        for id in ids {
            let path = Path::new(dir).join(&id).join(file);
            let text = read(&root.join(&path))?;
            let mut found = Problems::new(&path);
            match serde_saphyr::from_str::<Value>(&text) {
                Ok(instance) => {
                    for error in validator.iter_errors(&instance) {
                        let message = error.to_string();
                        found.push(field_path(error.instance_path()), SchemaProblem { message });
                    }
                    files.insert(path, instance);
                }
                Err(e) => {
                    let message = e.to_string();
                    found.push(FieldPath::root(), StrictProblem::Decode { message });
                }
            }
            problems.extend(found.into_vec());
        }
    }
    Ok((problems, files))
}

/// The validator of `schema/v1/<schema>`.
fn schema_validator(root: &Path, schema: &str) -> Result<jsonschema::Validator, String> {
    let path = root.join("schema/v1").join(schema);
    let schema: Value =
        serde_json::from_str(&read(&path)?).map_err(|e| format!("{}: {e}", path.display()))?;
    validator(&schema).map_err(|e| format!("{}: {e}", path.display()))
}

fn read(path: &Path) -> Result<String, String> {
    fs::read_to_string(path).map_err(|e| format!("{}: {e}", path.display()))
}

/// The problems of the components, each on its own and all together.
pub(crate) fn component_problems(catalog: &Catalog) -> Vec<Problem> {
    let mut problems = Vec::new();
    for (id, component) in &catalog.components {
        let mut found = Problems::new(&component_file(id));
        problems_of(catalog, id, component, &mut found);
        problems.extend(found.into_vec());
    }
    let mut together = Problems::new(Path::new("components"));
    for profile in Profile::ALL {
        let order_problem = catalog
            .components
            .iter()
            .filter(|(_, c)| c.mode(profile) != Mode::Unsupported)
            .find_map(|(id, _)| order::install_order(&catalog.components, id, profile).err());
        if let Some(error) = order_problem {
            together.push(
                FieldPath::root(),
                ComponentProblem::InstallOrder { profile, error },
            );
        }
    }
    problems.extend(together.into_vec());
    problems
}

fn problems_of(catalog: &Catalog, id: &str, component: &Component, found: &mut Problems) {
    let root = FieldPath::root();
    if !is_id(id) {
        found.push(root.clone(), ComponentProblem::Id);
    }
    // An installer reads a name in enable or disable as a component id first, so that id must
    // not hide another component's capability.
    if let Some(capability) = component.capability()
        && catalog
            .components
            .get(capability)
            .is_some_and(|other| other.capability() != Some(capability))
    {
        let capability = capability.to_owned();
        found.push(
            root.clone().key("provides").key("capability"),
            ComponentProblem::CapabilityIsComponent { capability },
        );
    }
    for (i, release) in component.releases.iter().enumerate() {
        if let Release::Manifest(r) = release
            && r.url_digests.len() != r.urls.len()
        {
            found.push(
                root.clone().key("releases").index(i).key("urlDigests"),
                ComponentProblem::UrlDigests { urls: r.urls.len() },
            );
        }
    }
    for (i, input) in component.inputs.iter().enumerate() {
        let at = root.clone().key("inputs").index(i);
        if input.path.is_some() && component.input_release(input).is_none() {
            found.push(
                at.clone().key("path"),
                ComponentProblem::InputWithoutRelease,
            );
        }
        if input.required
            && input.default.is_none()
            && input.test_value.is_none()
            && input.generate.is_none()
        {
            found.push(at, ComponentProblem::InputWithoutValue);
        }
    }
    provision_problems(component, found);
    if let Some(resources) = &component.resources {
        work_problems(resources, found);
    }
    contribution_problems(catalog, component, found);
}

/// Whether `id` names a component or template: lowercase words joined by single hyphens.
pub(crate) fn is_id(id: &str) -> bool {
    id.split('-').all(|word| {
        !word.is_empty()
            && word
                .bytes()
                .all(|b| b.is_ascii_lowercase() || b.is_ascii_digit())
    })
}

/// A document of `networkPolicies`: a `List` stands for its items, as `kubeobj.Decode` reads it.
#[derive(Deserialize)]
#[serde(untagged)]
enum Document {
    List { items: Vec<Object> },
    One(Object),
}

impl Document {
    fn objects(self) -> Vec<Object> {
        match self {
            Self::List { items } => items,
            Self::One(object) => vec![object],
        }
    }
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
fn provision_problems(component: &Component, found: &mut Problems) {
    let root = FieldPath::root();
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
        let at = root.clone().key("provides").key("registry").key("ca");
        let namespace = registry.ca.namespace.clone();
        found.push(
            at.key("namespace"),
            ComponentProblem::CaNamespace { namespace },
        );
    }
    for (i, crd) in component.internal_crds.iter().enumerate() {
        if !component.detection.crds.contains(crd) {
            let crd = crd.clone();
            found.push(
                root.clone().key("internalCRDs").index(i),
                ComponentProblem::InternalCrd { crd },
            );
        }
    }
    for (i, input) in component.inputs.iter().enumerate() {
        if input.generate == Some(Generate::ClusterIp) && input.path.is_none() {
            found.push(
                root.clone().key("inputs").index(i).key("generate"),
                ComponentProblem::ClusterIpWithoutPath,
            );
        }
    }
    let at = root.key("networkPolicies");
    let policies = component.network_policies.as_deref().unwrap_or_default();
    match serde_saphyr::from_multiple::<Option<Document>>(policies) {
        Err(e) => {
            let message = e.to_string();
            found.push(at, ComponentProblem::NetworkPolicies { message });
        }
        Ok(objects) => {
            for object in objects.into_iter().flatten().flat_map(Document::objects) {
                if object.kind != "NetworkPolicy"
                    || !object.metadata.namespace.as_deref().is_some_and(owns)
                {
                    found.push(
                        at.clone(),
                        ComponentProblem::NetworkPolicy {
                            kind: object.kind,
                            name: object.metadata.name,
                        },
                    );
                }
            }
        }
    }
}

fn work_problems(resources: &Resources, found: &mut Problems) {
    let mut seen = Vec::new();
    for (i, work) in resources.work.iter().enumerate() {
        let at = FieldPath::root().key("resources").key("work").index(i);
        if seen.contains(&work.name.as_str()) {
            let name = work.name.clone();
            found.push(at.clone().key("name"), ComponentProblem::WorkTwice { name });
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
                    let at = at.clone().key(field).key(resource);
                    found.push(at, ComponentProblem::Quantity(e));
                }
            }
        }
    }
}

/// A Kubernetes quantity [quantity], such as `256Mi` or `100m`.
///
/// [quantity]: https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/
const QUANTITY: &str =
    r"\A[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)(([KMGTPE]i)|[numkMGTPE]|([eE][+-]?[0-9]+))?\z";

/// Whether `value` is a quantity above zero.
fn above_zero(value: &str) -> Result<(), QuantityProblem> {
    let not_a_quantity = || QuantityProblem::NotAQuantity {
        value: value.to_owned(),
    };
    // QUANTITY is a constant that a test compiles, so it does not fail here.
    let quantity = Regex::new(QUANTITY).ok();
    let Some(number) = quantity
        .as_ref()
        .and_then(|q| q.captures(value))
        .and_then(|c| c.get(1))
    else {
        return Err(not_a_quantity());
    };
    if value.starts_with('-') || !number.as_str().bytes().any(|b| matches!(b, b'1'..=b'9')) {
        return Err(QuantityProblem::NotAboveZero {
            value: value.to_owned(),
        });
    }
    Ok(())
}

fn contribution_problems(catalog: &Catalog, component: &Component, found: &mut Problems) {
    let at = FieldPath::root().key("contributes");
    // An installer finds the contributors it installed by their Helm releases.
    if !component.contributes.is_empty() && !component.releases.iter().any(|r| r.helm().is_some()) {
        found.push(at.clone(), ComponentProblem::ContributorWithoutHelm);
    }
    for (i, contribution) in component.contributes.iter().enumerate() {
        let at = at.clone().index(i).key("target");
        let target = &contribution.target;
        match catalog.components.get(&target.component) {
            None => found.push(
                at.key("component"),
                ComponentProblem::UnknownTarget {
                    component: target.component.clone(),
                },
            ),
            Some(other) if other.release(&target.release).is_none() => found.push(
                at.key("release"),
                ComponentProblem::UnknownTargetRelease {
                    component: target.component.clone(),
                    release: target.release.clone(),
                },
            ),
            Some(_) => {}
        }
    }
}

#[cfg(test)]
mod tests {
    use serde_json::json;

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
    fn problems(id: &str, edit: impl FnOnce(&mut Component)) -> Vec<Problem> {
        let mut catalog = catalog();
        edit(catalog.components.get_mut(id).unwrap());
        component_problems(&catalog)
    }

    fn problem(file: PathBuf, at: FieldPath, kind: ComponentProblem) -> Problem {
        Problem {
            file,
            at,
            kind: kind.into(),
        }
    }

    #[test]
    fn the_schemas_assert_formats() {
        let schema: Value =
            serde_json::from_str(&read(&root().join("schema/v1/component.schema.json")).unwrap())
                .unwrap();
        let validator = validator(&schema).unwrap();
        let mut component: Value = serde_saphyr::from_str(
            &read(&root().join("components/traefik/component.yaml")).unwrap(),
        )
        .unwrap();
        assert!(validator.is_valid(&component));
        component["metadata"]["homepage"] = Value::from("not a URL");
        assert!(!validator.is_valid(&component));
    }

    #[test]
    fn a_template_memory_writes_kilo_as_lowercase_k() {
        let schema: Value =
            serde_json::from_str(&read(&root().join("schema/v1/template.schema.json")).unwrap())
                .unwrap();
        let validator = validator(&schema).unwrap();
        let mut template: Value = serde_saphyr::from_str(
            &read(&root().join("templates/browserless/template.yaml")).unwrap(),
        )
        .unwrap();
        template["services"][0]["resources"]["requests"]["memory"] = Value::from("64k");
        assert!(validator.is_valid(&template));
        template["services"][0]["resources"]["requests"]["memory"] = Value::from("64K");
        assert!(!validator.is_valid(&template));
    }

    #[test]
    fn a_capability_must_not_be_the_id_of_another_component() {
        let got = problems("traefik", |c| {
            c.provides.as_mut().unwrap().capability = Some("cert-manager".to_owned());
        });
        let want = problem(
            component_file("traefik"),
            FieldPath::root().key("provides").key("capability"),
            ComponentProblem::CapabilityIsComponent {
                capability: "cert-manager".to_owned(),
            },
        );
        assert!(got.contains(&want), "{got:?}");
    }

    #[test]
    fn a_contributor_needs_a_helm_release() {
        let got = problems("crowdsec", |c| c.releases.retain(|r| r.helm().is_none()));
        let want = problem(
            component_file("crowdsec"),
            FieldPath::root().key("contributes"),
            ComponentProblem::ContributorWithoutHelm,
        );
        assert!(got.contains(&want), "{got:?}");
    }

    #[test]
    fn a_requirement_must_name_a_component_on_every_profile() {
        let got = problems("traefik", |c| {
            c.requires = vec![zeabur_catalog_format::common::Reference::Component(
                "no-such-component".to_owned(),
            )];
        });
        let want = problem(
            PathBuf::from("components"),
            FieldPath::root(),
            ComponentProblem::InstallOrder {
                profile: Profile::SingleNode,
                error: order::OrderError::UnknownComponent {
                    by: "traefik".to_owned(),
                    id: "no-such-component".to_owned(),
                },
            },
        );
        assert!(got.contains(&want), "{got:?}");
    }

    #[test]
    fn provisions_only_its_own_crds_inputs_and_namespaces() {
        let ok = "kind: NetworkPolicy\nmetadata: {name: a, namespace: zot}\n";
        let cases: &[(&str, Option<&str>, bool)] = &[
            ("its own namespace", Some(ok), false),
            (
                "a List of policies in its own namespace",
                Some(
                    "kind: List\nitems:\n  - kind: NetworkPolicy\n    metadata: {name: a, namespace: zot}\n",
                ),
                false,
            ),
            (
                "a List with a policy in another namespace",
                Some(
                    "kind: List\nitems:\n  - kind: NetworkPolicy\n    metadata: {name: a, namespace: default}\n",
                ),
                true,
            ),
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
        let want = problem(
            component_file("zeabur-builder"),
            FieldPath::root()
                .key("provides")
                .key("registry")
                .key("ca")
                .key("namespace"),
            ComponentProblem::CaNamespace {
                namespace: "default".to_owned(),
            },
        );
        assert!(got.contains(&want), "{got:?}");
        let got = problems("zeabur-builder", |c| {
            c.internal_crds = vec!["other.example.com".to_owned()];
        });
        let want = problem(
            component_file("zeabur-builder"),
            FieldPath::root().key("internalCRDs").index(0),
            ComponentProblem::InternalCrd {
                crd: "other.example.com".to_owned(),
            },
        );
        assert!(got.contains(&want), "{got:?}");
    }

    /// A change to a health check.
    type Edit<'a> = dyn Fn(&mut Value) + 'a;

    /// Whether the schema accepts component `id` with `field` set to `value`.
    fn accepts(id: &str, field: &str, value: Value) -> bool {
        let path = root().join(format!("components/{id}/component.yaml"));
        let mut component: Value = serde_saphyr::from_str(&read(&path).unwrap()).unwrap();
        component[field] = value;
        schema_validator(root(), "component.schema.json")
            .unwrap()
            .is_valid(&component)
    }

    #[test]
    #[expect(
        clippy::disallowed_methods,
        reason = "the cases are files the schema must refuse, which no type can hold"
    )]
    fn a_health_check_names_its_objects_one_way() {
        let health = |edit: &dyn Fn(&mut Value)| {
            let mut check = json!({
                "name": "controller",
                "resource": {"kind": "Deployment", "name": "traefik", "namespace": "traefik"},
                "timeout": "60s",
                "advice": "Check the pods.",
            });
            edit(&mut check);
            json!([check])
        };
        let pod = |h: &mut Value| {
            h["resource"]["kind"] = json!("Pod");
            h["condition"] = json!("Ready");
        };
        let unname = |h: &mut Value| {
            h["resource"].as_object_mut().unwrap().remove("name");
        };
        let cases: &[(&str, bool, &Edit<'_>)] = &[
            ("by name", true, &|_| {}),
            ("by selector", true, &|h| {
                unname(h);
                h["resource"]["selector"] = json!({"app.kubernetes.io/name": "traefik"});
            }),
            ("all", true, &|h| {
                pod(h);
                unname(h);
                h["resource"]["all"] = json!(true);
            }),
            ("condition of a kind that is not a workload", true, &pod),
            ("cluster-scoped kind", true, &|h| {
                h["resource"] = json!({"kind": "GatewayClass", "name": "traefik"});
                h["condition"] = json!("Accepted");
            }),
            ("workload with a condition", false, &|h| {
                h["condition"] = json!("Available");
            }),
            ("all of a workload", false, &|h| {
                unname(h);
                h["resource"]["all"] = json!(true);
            }),
            ("unknown kind", false, &|h| {
                h["resource"]["kind"] = json!("ConfigMap");
            }),
            ("no condition", false, &|h| {
                pod(h);
                h.as_object_mut().unwrap().remove("condition");
            }),
            ("no advice", false, &|h| {
                h.as_object_mut().unwrap().remove("advice");
            }),
            ("no timeout", false, &|h| {
                h.as_object_mut().unwrap().remove("timeout");
            }),
            ("no object", false, &unname),
            ("name and all", false, &|h| {
                h["resource"]["all"] = json!(true);
            }),
            ("all false", false, &|h| {
                unname(h);
                h["resource"]["all"] = json!(false);
            }),
            ("empty selector", false, &|h| {
                unname(h);
                h["resource"]["selector"] = json!({});
            }),
            ("namespaced kind without namespace", false, &|h| {
                h["resource"].as_object_mut().unwrap().remove("namespace");
            }),
            ("cluster-scoped kind with namespace", false, &|h| {
                h["resource"] =
                    json!({"kind": "GatewayClass", "name": "traefik", "namespace": "traefik"});
            }),
            ("kubectl arguments", false, &|h| {
                h["wait"] = json!(["--for=condition=Ready", "pod"]);
            }),
        ];
        for (name, ok, edit) in cases {
            assert_eq!(accepts("traefik", "health", health(edit)), *ok, "{name}");
        }
    }

    #[test]
    fn a_smoke_wait_needs_a_condition_except_for_workloads() {
        let cases = [
            (
                "object with a condition",
                true,
                json!({"resource": "certificate/smoke", "namespace": "x", "condition": "condition=Ready"}),
            ),
            (
                "object without a condition",
                false,
                json!({"resource": "certificate/smoke", "namespace": "x"}),
            ),
            (
                "workload without a condition",
                true,
                json!({"resource": "deployment/web", "namespace": "x"}),
            ),
            (
                "workload with a condition",
                false,
                json!({"resource": "deployment/web", "namespace": "x", "condition": "condition=Available"}),
            ),
            (
                "workload with a jsonpath",
                true,
                json!({"resource": "deployment/web", "namespace": "x", "condition": "jsonpath={.metadata.generation}=2"}),
            ),
            (
                "kind alone without selector",
                false,
                json!({"resource": "pod", "condition": "condition=Ready"}),
            ),
            (
                "kind alone with selector",
                true,
                json!({"resource": "pod", "selector": {"a": "b"}, "condition": "condition=Ready"}),
            ),
            (
                "object with all",
                false,
                json!({"resource": "pod/x", "all": true, "condition": "condition=Ready"}),
            ),
        ];
        for (name, ok, wait) in cases {
            let smoke = json!([{"name": "x", "wait": [wait]}]);
            assert_eq!(accepts("cert-manager", "smoke", smoke), ok, "{name}");
        }
    }

    #[test]
    fn quantities_are_above_zero() {
        for ok in ["1", "100m", "0.5", "256Mi", "1e3", "+1"] {
            assert_eq!(above_zero(ok), Ok(()), "{ok}");
        }
        for bad in ["0", "0m", "-1", "0.0"] {
            let value = bad.to_owned();
            assert_eq!(
                above_zero(bad),
                Err(QuantityProblem::NotAboveZero { value })
            );
        }
        for bad in ["abc", "1Mb", ""] {
            let value = bad.to_owned();
            assert_eq!(
                above_zero(bad),
                Err(QuantityProblem::NotAQuantity { value })
            );
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
