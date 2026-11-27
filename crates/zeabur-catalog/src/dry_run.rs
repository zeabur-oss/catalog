//! `zeabur-catalog dry-run`: every object that `zeabur-catalog render` wrote, sent to a local API
//! server with `--dry-run=server`. The API server checks fields, defaults, CEL rules and Pod
//! Security; nothing runs. Then [`refusals`] checks what the API server refuses to update.

pub(crate) mod refusals;
pub(crate) mod server;

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::sync::atomic::{AtomicUsize, Ordering};

use k8s_openapi::apimachinery::pkg::apis::meta::v1::ObjectMeta;
use serde::Deserialize;
use serde_json::json;
use zeabur_catalog_format::Catalog;
use zeabur_catalog_format::common::{Mode, PodSecurity, Profile};
use zeabur_catalog_format::component::default_accounts;
use zeabur_catalog_format::problem::{DryRunProblem, FieldPath, Problem};

use self::server::{ApiServer, Kubectl};
use crate::render::{documents, kube_versions};

/// How many `kubectl apply` run at once.
const WORKERS: usize = 8;

/// One object of a rendered release.
#[derive(Debug)]
pub(crate) struct Doc {
    /// Its kind.
    pub(crate) kind: String,
    /// Its name.
    pub(crate) name: String,
    /// Its namespace, or the release's when it names none.
    pub(crate) namespace: String,
    /// A Helm hook.
    pub(crate) hook: bool,
    /// The object as JSON.
    pub(crate) text: String,
}

/// The objects in `data`, a rendered release in `namespace`. A document without a kind is left
/// out.
pub(crate) fn docs(data: &[u8], namespace: &str) -> Result<Vec<Doc>, String> {
    #[derive(Deserialize)]
    struct Head {
        #[serde(default)]
        kind: String,
        #[serde(default)]
        metadata: ObjectMeta,
    }
    let mut out = Vec::new();
    for doc in documents(data)? {
        let text = serde_json::to_string(&doc).map_err(|e| e.to_string())?;
        let head: Head = serde_json::from_value(doc).map_err(|e| e.to_string())?;
        if head.kind.is_empty() {
            continue;
        }
        let meta = head.metadata;
        out.push(Doc {
            kind: head.kind,
            name: meta.name.unwrap_or_default(),
            namespace: meta
                .namespace
                .filter(|n| !n.is_empty())
                .unwrap_or_else(|| namespace.to_owned()),
            hook: meta
                .annotations
                .is_some_and(|a| a.contains_key("helm.sh/hook")),
            text,
        });
    }
    Ok(out)
}

/// The objects joined into one stream per namespace, because `kubectl -n` refuses objects that
/// name another namespace.
pub(crate) fn by_namespace<'a>(
    docs: impl IntoIterator<Item = &'a Doc>,
) -> BTreeMap<String, String> {
    let mut joined: BTreeMap<String, String> = BTreeMap::new();
    for doc in docs {
        let text = joined.entry(doc.namespace.clone()).or_default();
        text.push_str(&doc.text);
        text.push_str("\n---\n");
    }
    joined
}

/// Objects to send in one `kubectl apply`.
#[derive(Debug)]
struct Send {
    /// The file a refusal is reported in.
    file: PathBuf,
    /// What they are, such as `single-node postgresql/cnpg`.
    subject: String,
    namespace: String,
    text: String,
    /// A warning fails them too. Pod Security warns on a workload whose pods its enforce level
    /// rejects.
    strict: bool,
}

/// What the dry run sends, and what it creates first.
#[derive(Debug, Default)]
struct Plan {
    sends: Vec<Send>,
    /// The CRDs by name.
    crds: BTreeMap<String, String>,
    /// The namespaces the objects name.
    namespaces: BTreeSet<String>,
    /// The namespaces of the templates, with their Pod Security labels.
    #[expect(
        clippy::disallowed_types,
        reason = "Namespaces as kubectl applies them"
    )]
    labelled: BTreeMap<String, serde_json::Value>,
}

/// Starts an API server, sends it everything `zeabur-catalog render` wrote under `render` for
/// the Kubernetes version with the server's minor, then checks the refusals. It returns the
/// problems.
pub(crate) fn dry_run(root: &Path, render: &Path) -> Result<Vec<Problem>, String> {
    let catalog = Catalog::read(root).map_err(|e| e.to_string())?;
    let versions = kube_versions(root)?;
    let server = ApiServer::start(&root.join("out/apiserver"))?;
    let running = server.version()?;
    let version = versions
        .iter()
        .find(|v| minor(v) == minor(&running))
        .ok_or_else(|| {
            format!(
                "the API server is Kubernetes {running}; config/kubernetes.yaml renders no \
                 version with the same minor"
            )
        })?;
    let dir = render.join(version);
    let mut plan = Plan::default();
    components(&catalog, &dir, &mut plan)?;
    templates(root, &catalog, &dir, &mut plan)?;
    prepare(&plan, &server.kubectl)?;
    let problems = send(&plan.sends, &server.kubectl)?;
    if !problems.is_empty() {
        return Ok(problems);
    }
    refusals::check(root, &catalog, &dir, &server.kubectl)
}

/// `1.37` of `1.37.2`.
pub(crate) fn minor(version: &str) -> &str {
    match version.match_indices('.').nth(1) {
        Some((i, _)) => version.get(..i).unwrap_or(version),
        None => version,
    }
}

/// The documents as one YAML stream.
pub(crate) fn stream<'a>(docs: impl IntoIterator<Item = &'a str>) -> String {
    docs.into_iter().fold(String::new(), |mut all, doc| {
        all.push_str(doc);
        all.push_str("\n---\n");
        all
    })
}

/// Adds every release that each profile renders under `dir`, and each component's network
/// policies.
fn components(catalog: &Catalog, dir: &Path, plan: &mut Plan) -> Result<(), String> {
    for profile in Profile::ALL {
        for (id, component) in &catalog.components {
            if *component.defaults.get(profile) == Mode::Unsupported {
                continue;
            }
            let file = Path::new("components").join(id).join("component.yaml");
            let accounts = default_accounts(component);
            for release in accounts.iter().chain(&component.releases) {
                let (Some(name), Some(namespace)) = (release.name(), release.namespace()) else {
                    continue;
                };
                let path = dir
                    .join(profile.as_str())
                    .join(id)
                    .join(format!("{name}.yaml"));
                let data = fs::read(&path).map_err(|e| format!("{}: {e}", path.display()))?;
                plan.namespaces.insert(namespace.to_owned());
                let docs = docs(&data, namespace).map_err(|e| format!("{id}/{name}: {e}"))?;
                for doc in &docs {
                    plan.namespaces.insert(doc.namespace.clone());
                    match doc.kind.as_str() {
                        "CustomResourceDefinition" => {
                            plan.crds.insert(doc.name.clone(), doc.text.clone());
                        }
                        "Namespace" => {
                            plan.namespaces.insert(doc.name.clone());
                        }
                        _ => {}
                    }
                }
                for (namespace, text) in by_namespace(&docs) {
                    plan.sends.push(Send {
                        file: file.clone(),
                        subject: format!("{} {id}/{name}", profile.as_str()),
                        namespace,
                        text,
                        strict: false,
                    });
                }
            }
        }
    }
    for (id, component) in &catalog.components {
        let Some(policies) = &component.network_policies else {
            continue;
        };
        let docs =
            docs(policies.as_bytes(), "").map_err(|e| format!("{id} networkPolicies: {e}"))?;
        for (namespace, text) in by_namespace(&docs) {
            plan.sends.push(Send {
                file: Path::new("components").join(id).join("component.yaml"),
                subject: format!("{id} networkPolicies"),
                namespace,
                text,
                strict: false,
            });
        }
    }
    Ok(())
}

/// Adds each template as its e2e run applies it: the objects of its `e2e.yaml` `setup`, its
/// render under `dir`, and the Secrets of its generated inputs, in a namespace with the Pod
/// Security level of its services.
fn templates(root: &Path, catalog: &Catalog, dir: &Path, plan: &mut Plan) -> Result<(), String> {
    for (id, template) in &catalog.templates {
        let run = crate::render::template::e2e(root, id, template)
            .map_err(|e| format!("templates/{id}: {e}"))?;
        let level = template
            .template
            .services
            .iter()
            .map(|s| s.pod_security)
            .max()
            .unwrap_or(PodSecurity::Restricted)
            .as_str();
        let namespace = json!({
            "apiVersion": "v1",
            "kind": "Namespace",
            "metadata": {"name": run.namespace, "labels": {
                "pod-security.kubernetes.io/enforce": level,
                "pod-security.kubernetes.io/audit": "restricted",
                "pod-security.kubernetes.io/warn": level,
            }},
        });
        plan.labelled.insert(run.namespace.clone(), namespace);
        let path = dir.join("templates").join(format!("{id}.yaml"));
        let rendered = fs::read_to_string(&path).map_err(|e| format!("{}: {e}", path.display()))?;
        let mut texts = vec![run.setup.clone(), rendered];
        if template
            .template
            .inputs
            .iter()
            .any(|i| i.generate.is_some())
        {
            let list =
                crate::secrets::inputs(id, template, &run.namespace, crate::secrets::os_random)?;
            texts.push(serde_json::to_string(&list).map_err(|e| e.to_string())?);
        }
        for text in texts.into_iter().filter(|t| !t.trim().is_empty()) {
            plan.sends.push(Send {
                file: Path::new("templates").join(id).join("manifests.yaml"),
                subject: format!("templates/{id}"),
                namespace: run.namespace.clone(),
                text,
                strict: true,
            });
        }
        plan.namespaces.insert(run.namespace);
    }
    Ok(())
}

/// Applies the CRDs and waits until they are established, then creates the namespaces, each
/// with a default `ServiceAccount`.
fn prepare(plan: &Plan, kubectl: &Kubectl) -> Result<(), String> {
    crate::output::info(&format!("applying {} CRDs", plan.crds.len()));
    let crds = stream(plan.crds.values().map(String::as_str));
    kubectl.text(
        crds.as_bytes(),
        &["apply", "--server-side", "--force-conflicts", "-f", "-"],
    )?;
    kubectl.text(
        &[],
        &[
            "wait",
            "--for=condition=Established",
            "crd",
            "--all",
            "--timeout=2m",
        ],
    )?;
    let names = plan.namespaces.iter().filter(|n| !n.is_empty());
    let mut namespaces: Vec<_> = names
        .clone()
        .filter(|n| !plan.labelled.contains_key(*n))
        .map(|n| json!({"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": n}}))
        .collect();
    namespaces.extend(plan.labelled.values().cloned());
    apply_list(kubectl, &namespaces)?;
    // The API server has no controller manager to create each namespace's default
    // ServiceAccount, which Pods without one use.
    let accounts: Vec<_> = names
        .map(|n| {
            json!({"apiVersion": "v1", "kind": "ServiceAccount",
                   "metadata": {"name": "default", "namespace": n}})
        })
        .collect();
    apply_list(kubectl, &accounts)
}

/// Sends each of `sends` with `--dry-run=server`, [`WORKERS`] at once, and returns a problem
/// for each the API server refuses.
fn send(sends: &[Send], kubectl: &Kubectl) -> Result<Vec<Problem>, String> {
    let next = AtomicUsize::new(0);
    let found = Mutex::new(Vec::new());
    std::thread::scope(|s| {
        for _ in 0..WORKERS {
            s.spawn(|| {
                while let Some(send) = sends.get(next.fetch_add(1, Ordering::Relaxed)) {
                    let mut args = vec![
                        "apply",
                        "--server-side",
                        "--force-conflicts",
                        "--dry-run=server",
                        "--field-manager=catalog",
                    ];
                    if !send.namespace.is_empty() {
                        args.extend(["-n", &send.namespace]);
                    }
                    args.extend(["-f", "-"]);
                    if send.strict {
                        args.push("--warnings-as-errors");
                    }
                    if let Err(failed) = kubectl.run(send.text.as_bytes(), &args) {
                        let problem = Problem {
                            file: send.file.clone(),
                            at: FieldPath::root(),
                            kind: DryRunProblem::Refused {
                                subject: send.subject.clone(),
                                output: failed.stderr,
                            }
                            .into(),
                        };
                        if let Ok(mut found) = found.lock() {
                            found.push(problem);
                        }
                    }
                }
            });
        }
    });
    crate::output::info(&format!("dry run done: {} manifests", sends.len()));
    let mut found = found.into_inner().map_err(|e| e.to_string())?;
    found.sort_by_key(ToString::to_string);
    Ok(found)
}

/// Applies `items` as one `List`.
#[expect(
    clippy::disallowed_types,
    reason = "Namespaces and ServiceAccounts as kubectl applies them"
)]
fn apply_list(kubectl: &Kubectl, items: &[serde_json::Value]) -> Result<(), String> {
    let list = json!({"apiVersion": "v1", "kind": "List", "items": items});
    let text = serde_json::to_string(&list).map_err(|e| e.to_string())?;
    kubectl
        .text(text.as_bytes(), &["apply", "-f", "-"])
        .map(drop)
}

#[cfg(test)]
mod tests {
    #[test]
    fn minor_drops_the_patch() {
        assert_eq!(super::minor("1.37.2"), "1.37");
        assert_eq!(super::minor("1.37"), "1.37");
    }

    #[test]
    fn docs_take_the_release_namespace_and_mark_hooks() {
        let data = b"apiVersion: v1\nkind: ConfigMap\nmetadata: {name: a}\n---\n# empty\n---\n\
                     apiVersion: batch/v1\nkind: Job\nmetadata: {name: b, namespace: other, \
                     annotations: {helm.sh/hook: pre-upgrade}}\n";
        let docs = super::docs(data, "db").unwrap();
        assert_eq!(docs.len(), 2);
        assert_eq!((docs[0].namespace.as_str(), docs[0].hook), ("db", false));
        assert_eq!((docs[1].namespace.as_str(), docs[1].hook), ("other", true));
        let joined = super::by_namespace(&docs);
        assert_eq!(joined.keys().collect::<Vec<_>>(), ["db", "other"]);
    }
}
