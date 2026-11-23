//! `zeabur-catalog api`: the static API of a catalog release, `catalog.json`,
//! `components/<id>.json`, `templates/<id>.json` and the schemas, checked against the schemas.
#![expect(
    clippy::disallowed_types,
    reason = "the API publishes each file as written, and checks its files and schemas as JSON"
)]

use std::collections::BTreeMap;
use std::fs;
use std::path::Path;
use std::process::Command;

use jsonschema::{Retrieve, Uri};
use k8s_openapi::api::core::v1::{Container, PersistentVolumeClaim, PodTemplateSpec};
use k8s_openapi::apimachinery::pkg::api::resource::Quantity;
use k8s_openapi::apimachinery::pkg::apis::meta::v1::ObjectMeta;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use zeabur_catalog_engine::digest::source_digest;
use zeabur_catalog_engine::quantity;
use zeabur_catalog_format::Catalog;
use zeabur_catalog_format::api::{
    CatalogRelease, Entry, Environment, Index, MeasurementFile, PinnedDistribution, Render,
    RenderedClaim, RenderedContainer, RenderedProfile, RenderedWorkload,
};
use zeabur_catalog_format::common::{Distribution, Mode, Profile, WorkloadKind};
use zeabur_catalog_format::component::{Component, Release};
use zeabur_catalog_format::contributions::Contributions;
use zeabur_catalog_format::problem::{Problem, ProblemKind, Problems, SchemaProblem};

use crate::check::field_path;
use crate::render::{Cache, documents, kube_versions};

/// The N of the `/vN/` path. It changes only on an incompatible change.
pub(crate) const MAJOR: u32 = 1;

/// The schemas the API publishes under `schema/`.
const SCHEMAS: [&str; 5] = [
    "catalog.schema.json",
    "component-file.schema.json",
    "component.schema.json",
    "template-file.schema.json",
    "template.schema.json",
];

/// The release of each distribution in `config/kubernetes.yaml` that e2e installs.
#[derive(Deserialize)]
struct Pins {
    k3s: Pin,
    rke2: Pin,
    kubesolo: Pin,
}

#[derive(Deserialize)]
struct Pin {
    version: String,
}

/// A [`ComponentFile`](zeabur_catalog_format::api::ComponentFile) whose declaration is
/// `component.yaml` as written, so it has no field the author left out.
#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct WrittenComponent<'a> {
    id: &'a str,
    source_digest: &'a str,
    declaration: Value,
    render: Render,
    measurements: Vec<Environment>,
    readmes: Vec<String>,
}

/// A [`TemplateFile`](zeabur_catalog_format::api::TemplateFile) whose template is `template.yaml`
/// as written.
#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct WrittenTemplate<'a> {
    id: &'a str,
    source_digest: &'a str,
    template: Value,
    manifests: &'a str,
    measurements: Vec<Environment>,
    readmes: Vec<String>,
}

/// Replaces `dir` with the API of the checkout at `root`, for the catalog `version` at `commit`.
pub(crate) fn write(root: &Path, version: &str, commit: &str, dir: &Path) -> Result<(), String> {
    let catalog = Catalog::read(root).map_err(|e| e.to_string())?;
    let kubernetes = kube_versions(root)?;
    let kube_version = kubernetes
        .last()
        .ok_or("config/kubernetes.yaml: no versions")?;
    let pins: Pins = read_yaml(&root.join("config/kubernetes.yaml"))?;
    let contributions = Contributions::of(
        &catalog.components,
        catalog.components.keys().map(String::as_str),
    )
    .map_err(|e| e.to_string())?;
    let cache = Cache::user()?;
    if dir.exists() {
        fs::remove_dir_all(dir).map_err(|e| format!("{}: {e}", dir.display()))?;
    }
    let mut index = Index {
        catalog: CatalogRelease {
            version: version.to_owned(),
            commit: commit.to_owned(),
        },
        profiles: Profile::ALL.to_vec(),
        kubernetes: kubernetes.clone(),
        distributions: [
            (Distribution::K3s, pins.k3s),
            (Distribution::Rke2, pins.rke2),
            (Distribution::Kubesolo, pins.kubesolo),
        ]
        .into_iter()
        .map(|(name, pin)| PinnedDistribution {
            name,
            version: pin.version,
        })
        .collect(),
        components: Vec::new(),
        templates: Vec::new(),
    };
    for (id, component) in &catalog.components {
        let entry = entry(root, "components", id)?;
        let render = Render {
            kubernetes_version: kube_version.clone(),
            profiles: rendered(&cache, component, id, kube_version, &contributions)
                .map_err(|e| format!("{id}: {e}"))?,
        };
        let file = WrittenComponent {
            id: &entry.id,
            source_digest: &entry.source_digest,
            declaration: read_yaml(&root.join(format!("components/{id}/component.yaml")))?,
            render,
            measurements: measurements(&root.join(format!("measurements/components/{id}.json")))?,
            readmes: readmes(
                &root.join("components").join(id),
                &dir.join("components").join(id),
            )?,
        };
        write_json(&dir.join(format!("components/{id}.json")), &file)?;
        index.components.push(entry);
        crate::output::info(&format!("wrote {id}"));
    }
    for (id, template) in &catalog.templates {
        let entry = entry(root, "templates", id)?;
        let file = WrittenTemplate {
            id: &entry.id,
            source_digest: &entry.source_digest,
            template: read_yaml(&root.join(format!("templates/{id}/template.yaml")))?,
            manifests: &template.manifests,
            measurements: measurements(&root.join(format!("measurements/templates/{id}.json")))?,
            readmes: readmes(
                &root.join("templates").join(id),
                &dir.join("templates").join(id),
            )?,
        };
        write_json(&dir.join(format!("templates/{id}.json")), &file)?;
        index.templates.push(entry);
    }
    write_json(&dir.join("catalog.json"), &index)?;
    let schemas = dir.join("schema");
    fs::create_dir_all(&schemas).map_err(|e| format!("{}: {e}", schemas.display()))?;
    for name in SCHEMAS {
        let from = root.join("schema/v1").join(name);
        fs::copy(&from, schemas.join(name)).map_err(|e| format!("{}: {e}", from.display()))?;
    }
    Ok(())
}

/// The schemas of an API, by file name.
#[derive(Clone)]
pub(crate) struct Schemas(BTreeMap<&'static str, Value>);

impl Schemas {
    /// The schemas in `dir`.
    pub(crate) fn read(dir: &Path) -> Result<Self, String> {
        SCHEMAS
            .into_iter()
            .map(|name| Ok((name, read_json(&dir.join(name))?)))
            .collect::<Result<_, String>>()
            .map(Self)
    }

    /// Each schema changed by `f`.
    pub(crate) fn map(self, f: impl Fn(Value) -> Value) -> Self {
        Self(
            self.0
                .into_iter()
                .map(|(name, schema)| (name, f(schema)))
                .collect(),
        )
    }

    /// A validator of `name`. Its `$ref`s name the other schemas, whatever host their `$id`
    /// names. It asserts `format`, which Draft 2020-12 only annotates by default.
    fn validator(&self, name: &str) -> Result<jsonschema::Validator, String> {
        let schema = self
            .0
            .get(name)
            .ok_or_else(|| format!("no schema {name}"))?;
        jsonschema::options()
            .should_validate_formats(true)
            .with_retriever(self.clone())
            .build(schema)
            .map_err(|e| format!("{name}: {e}"))
    }
}

impl Retrieve for Schemas {
    fn retrieve(
        &self,
        uri: &Uri<String>,
    ) -> Result<Value, Box<dyn std::error::Error + Send + Sync>> {
        let name = uri.path().as_str().rsplit('/').next().unwrap_or_default();
        Ok(self
            .0
            .get(name)
            .cloned()
            .ok_or_else(|| format!("{uri}: not a schema of the API"))?)
    }
}

/// The problems of each file of the API in `dir` against `schemas`.
pub(crate) fn check(dir: &Path, schemas: &Schemas) -> Result<Vec<Problem>, String> {
    check_as(dir, schemas, ProblemKind::from)
}

/// The problems of each file of the API in `dir` against `schemas`, each of the kind `kind` makes
/// of what the schema finds.
pub(crate) fn check_as(
    dir: &Path,
    schemas: &Schemas,
    kind: impl Fn(SchemaProblem) -> ProblemKind,
) -> Result<Vec<Problem>, String> {
    let mut files = vec![("catalog.schema.json", dir.join("catalog.json"))];
    for (kind, schema) in [
        ("components", "component-file.schema.json"),
        ("templates", "template-file.schema.json"),
    ] {
        let kind = dir.join(kind);
        let entries = fs::read_dir(&kind).map_err(|e| format!("{}: {e}", kind.display()))?;
        for entry in entries {
            let path = entry
                .map_err(|e| format!("{}: {e}", kind.display()))?
                .path();
            if path.extension().is_some_and(|e| e == "json") {
                files.push((schema, path));
            }
        }
    }
    files.sort();
    let mut problems = Vec::new();
    for (schema, path) in files {
        let validator = schemas.validator(schema)?;
        let doc: Value = read_json(&path)?;
        let mut found = Problems::new(&path);
        for error in validator.iter_errors(&doc) {
            let message = error.to_string();
            found.push(
                field_path(error.instance_path()),
                kind(SchemaProblem { message }),
            );
        }
        problems.extend(found.into_vec());
    }
    Ok(problems)
}

/// The objects of each release of `component` on each profile it supports.
fn rendered(
    cache: &Cache,
    component: &Component,
    id: &str,
    kube_version: &str,
    contributions: &Contributions,
) -> Result<BTreeMap<Profile, RenderedProfile>, String> {
    let mut profiles = BTreeMap::new();
    for profile in Profile::ALL {
        if *component.defaults.get(profile) == Mode::Unsupported {
            continue;
        }
        let mut rendered = RenderedProfile {
            workloads: Vec::new(),
            persistent_volume_claims: Vec::new(),
        };
        for release in &component.releases {
            let objects =
                cache.render(component, release, profile, kube_version, contributions, id)?;
            let name = release.name().unwrap_or_default();
            read_objects(&objects, release, &mut rendered)
                .map_err(|e| format!("{name} {profile}: {e}"))?;
        }
        profiles.insert(profile, rendered);
    }
    Ok(profiles)
}

#[derive(Deserialize)]
struct Head {
    #[serde(default)]
    kind: String,
    #[serde(default)]
    metadata: ObjectMeta,
}

#[derive(Deserialize)]
struct Workload {
    spec: WorkloadSpec,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct WorkloadSpec {
    replicas: Option<i32>,
    template: PodTemplateSpec,
    #[serde(default)]
    volume_claim_templates: Vec<PersistentVolumeClaim>,
}

/// Adds the workloads and `PersistentVolumeClaims` of `objects`, the objects of `release`.
fn read_objects(
    objects: &[u8],
    release: &Release,
    profile: &mut RenderedProfile,
) -> Result<(), String> {
    let release_name = release.name().unwrap_or_default().to_owned();
    for doc in documents(objects)? {
        let head: Head = serde_json::from_value(doc.clone()).map_err(|e| e.to_string())?;
        let name = head.metadata.name.unwrap_or_default();
        let namespace = head
            .metadata
            .namespace
            .unwrap_or_else(|| release.namespace().unwrap_or_default().to_owned());
        if head.kind == "PersistentVolumeClaim" {
            let pvc: PersistentVolumeClaim =
                serde_json::from_value(doc).map_err(|e| format!("{name}: {e}"))?;
            profile.persistent_volume_claims.push(RenderedClaim {
                release: Some(release_name.clone()),
                namespace: Some(namespace),
                name,
                storage: storage(&pvc)?,
            });
            continue;
        }
        let kind = match head.kind.as_str() {
            "Deployment" => WorkloadKind::Deployment,
            "StatefulSet" => WorkloadKind::StatefulSet,
            "DaemonSet" => WorkloadKind::DaemonSet,
            _ => continue,
        };
        let workload: Workload = serde_json::from_value(doc).map_err(|e| format!("{name}: {e}"))?;
        let spec = workload.spec;
        let pod = spec.template.spec.unwrap_or_default();
        profile.workloads.push(RenderedWorkload {
            release: release_name.clone(),
            namespace,
            name,
            kind,
            // A DaemonSet runs one pod on each node; the others default to one replica.
            replicas: (kind != WorkloadKind::DaemonSet).then(|| spec.replicas.unwrap_or(1)),
            containers: containers(&pod.containers)?,
            init_containers: containers(pod.init_containers.as_deref().unwrap_or_default())?,
            volume_claim_templates: if kind == WorkloadKind::StatefulSet {
                spec.volume_claim_templates
                    .iter()
                    .map(|claim| {
                        Ok(RenderedClaim {
                            release: None,
                            namespace: None,
                            name: claim.metadata.name.clone().unwrap_or_default(),
                            storage: storage(claim)?,
                        })
                    })
                    .collect::<Result<_, String>>()?
            } else {
                Vec::new()
            },
        });
    }
    Ok(())
}

fn containers(list: &[Container]) -> Result<Vec<RenderedContainer>, String> {
    list.iter()
        .map(|c| {
            let resources = c.resources.clone().unwrap_or_default();
            Ok(RenderedContainer {
                name: c.name.clone(),
                requests: quantities(resources.requests.unwrap_or_default())?,
                limits: quantities(resources.limits.unwrap_or_default())?,
            })
        })
        .collect()
}

fn quantities(list: BTreeMap<String, Quantity>) -> Result<BTreeMap<String, String>, String> {
    list.into_iter()
        .map(|(name, q)| Ok((name, canonical(&q)?)))
        .collect()
}

/// The requested storage of a claim, `0` if it requests none.
fn storage(claim: &PersistentVolumeClaim) -> Result<String, String> {
    claim
        .spec
        .as_ref()
        .and_then(|spec| spec.resources.as_ref())
        .and_then(|r| r.requests.as_ref())
        .and_then(|r| r.get("storage"))
        .map_or_else(|| Ok("0".to_owned()), canonical)
}

fn canonical(q: &Quantity) -> Result<String, String> {
    quantity::canonical(&q.0).ok_or_else(|| format!("{}: not a quantity", q.0))
}

/// Every recorded run in the measurement file at `path`, none if there is no file.
fn measurements(path: &Path) -> Result<Vec<Environment>, String> {
    if !path.exists() {
        return Ok(Vec::new());
    }
    let file: MeasurementFile = read_json(path)?;
    Ok(file.environments)
}

/// Copies the READMEs of the directory `from` to `to` under the same names and returns their
/// locales: `en` for `README.md`, `zh-TW` for `README.zh-TW.md`.
fn readmes(from: &Path, to: &Path) -> Result<Vec<String>, String> {
    let entries = fs::read_dir(from).map_err(|e| format!("{}: {e}", from.display()))?;
    let mut locales = Vec::new();
    for entry in entries {
        let entry = entry.map_err(|e| format!("{}: {e}", from.display()))?;
        let name = entry.file_name();
        let Some(locale) = name.to_str().and_then(readme_locale) else {
            continue;
        };
        fs::create_dir_all(to).map_err(|e| format!("{}: {e}", to.display()))?;
        fs::copy(entry.path(), to.join(&name))
            .map_err(|e| format!("{}: {e}", entry.path().display()))?;
        locales.push(locale.to_owned());
    }
    locales.sort();
    Ok(locales)
}

/// The locale of a README file name, `None` for another file.
fn readme_locale(name: &str) -> Option<&str> {
    let stem = name.strip_prefix("README")?.strip_suffix(".md")?;
    if stem.is_empty() {
        return Some("en");
    }
    stem.strip_prefix('.').filter(|l| !l.is_empty())
}

fn entry(root: &Path, kind: &str, id: &str) -> Result<Entry, String> {
    let dir = root.join(kind).join(id);
    Ok(Entry {
        id: id.to_owned(),
        source_digest: source_digest(&dir).map_err(|e| format!("{}: {e}", dir.display()))?,
    })
}

/// The commit of the checkout at `root`.
pub(crate) fn commit(root: &Path) -> Result<String, String> {
    let out = Command::new("git")
        .args(["rev-parse", "HEAD"])
        .current_dir(root)
        .output()
        .map_err(|e| format!("git: {e}"))?;
    if !out.status.success() {
        return Err(format!(
            "git rev-parse HEAD: {}",
            String::from_utf8_lossy(&out.stderr).trim()
        ));
    }
    Ok(String::from_utf8_lossy(&out.stdout).trim().to_owned())
}

fn read_yaml<T: for<'de> Deserialize<'de>>(path: &Path) -> Result<T, String> {
    let text = fs::read_to_string(path).map_err(|e| format!("{}: {e}", path.display()))?;
    serde_saphyr::from_str(&text).map_err(|e| format!("{}: {e}", path.display()))
}

fn read_json<T: for<'de> Deserialize<'de>>(path: &Path) -> Result<T, String> {
    let text = fs::read_to_string(path).map_err(|e| format!("{}: {e}", path.display()))?;
    serde_json::from_str(&text).map_err(|e| format!("{}: {e}", path.display()))
}

fn write_json(path: &Path, value: &impl Serialize) -> Result<(), String> {
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir).map_err(|e| format!("{}: {e}", dir.display()))?;
    }
    let mut text = serde_json::to_string_pretty(value).map_err(|e| e.to_string())?;
    text.push('\n');
    fs::write(path, text).map_err(|e| format!("{}: {e}", path.display()))
}

#[cfg(test)]
pub(crate) mod tests {
    #![expect(
        clippy::disallowed_methods,
        reason = "the cases break a published file at a JSON pointer"
    )]

    use std::path::PathBuf;

    use serde_json::json;
    use zeabur_catalog_format::api::{ComponentFile, TemplateFile};

    use super::*;

    const WEB: &str = r#"apiVersion: catalog.zeabur.com/v1
kind: Component
metadata:
  name: Web
  summary: Serves a page.
  locales: {}
  license: Apache-2.0
  homepage: https://example.com
defaults: {single-node: recommended, multi-node: unsupported}
distributions: [k3s]
kubeVersion: ">=1.30.0-0"
releases:
  - name: web
    type: inline
    namespace: web
    manifests: |
      apiVersion: apps/v1
      kind: Deployment
      metadata: {name: web}
      spec:
        replicas: 2
        template:
          spec:
            containers:
              - name: web
                resources: {requests: {memory: 64Mi, cpu: 10m}, limits: {memory: 1024Mi, cpu: 0.5}}
      ---
      apiVersion: apps/v1
      kind: StatefulSet
      metadata: {name: db}
      spec:
        template:
          spec:
            containers: [{name: db}]
        volumeClaimTemplates:
          - metadata: {name: data}
            spec: {resources: {requests: {storage: 1Gi}}}
      ---
      apiVersion: apps/v1
      kind: DaemonSet
      metadata: {name: agent, namespace: kube-system}
      spec:
        template:
          spec:
            containers: [{name: agent}]
      ---
      apiVersion: v1
      kind: PersistentVolumeClaim
      metadata: {name: cache}
      spec: {resources: {requests: {storage: 5Gi}}}
detection: {crds: [webs.example.com]}
security: {summary: Reads nothing., podSecurity: restricted}
"#;

    const BLOG: &str = r#"apiVersion: catalog.zeabur.com/v1
kind: Template
metadata:
  name: Blog
  version: "1.0.0"
  summary: A blog.
  locales: {}
  license: MIT
  homepage: https://example.com
  docs: https://example.com/docs
services:
  - name: blog
    podSecurity: restricted
    ports: [{name: http, port: 8080}]
    resources: {requests: {cpu: 10m, memory: 64Mi}, limits: {cpu: 100m, memory: 128Mi}}
security: {summary: Reads nothing., risks: []}
smoke:
  - {name: ready, resource: deployment/blog, timeout: 60s}
"#;

    /// Not YAML until the installer expands its template field.
    const BLOG_MANIFESTS: &str = "# The blog.\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: blog\n  namespace: '{{ .Namespace }}'\n";

    const KUBERNETES: &str = r#"versions: ["1.36.0", "1.37.0"]
k3s: {version: v1.37.2+k3s1}
rke2: {version: v1.37.2+rke2r1}
kubesolo: {version: v1.3.0}
"#;

    const RUN: &str = r#"{"component": "web", "environments": [{"distribution": "k3s",
        "profile": "single-node", "platform": {"os": "linux", "architecture": "amd64"},
        "kubernetesVersion": "v1.37.2", "distributionVersion": "v1.37.2+k3s1", "result": "passed",
        "pods": [{"workload": "web", "kind": "Deployment", "memory": "30Mi", "cpu": "0"}],
        "versions": [{"release": "web", "version": "v1.0.0"}], "measuredAt": "2026-11-20"}]}"#;

    const BLOG_RUN: &str = r#"{"template": "blog", "environments": [{"distribution": "kind",
        "profile": "single-node", "platform": {"os": "linux", "architecture": "amd64"},
        "result": "passed", "pods": [{"workload": "blog", "kind": "Deployment", "memory": "80Mi",
        "cpu": "1m"}], "measuredAt": "2026-11-21"}]}"#;

    /// A catalog with the component `web`, the template `blog`, a run and READMEs of each, and its
    /// API in `out/v1`.
    pub(crate) struct Api(pub(crate) PathBuf);

    impl Api {
        pub(crate) fn new(name: &str) -> Self {
            let root = std::env::temp_dir()
                .join(format!("zeabur-catalog-api-{}-{name}", std::process::id()));
            fs::remove_dir_all(&root).unwrap_or_default();
            let schemas = Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../../schema/v1"));
            for (path, text) in [
                ("components/web/component.yaml", WEB),
                ("templates/blog/template.yaml", BLOG),
                ("templates/blog/manifests.yaml", BLOG_MANIFESTS),
                ("measurements/components/web.json", RUN),
                ("measurements/templates/blog.json", BLOG_RUN),
                ("components/web/README.md", "# Web\n"),
                ("components/web/README.zh-TW.md", "# Web 網頁\n"),
                ("templates/blog/README.md", "# Blog\n"),
                ("config/kubernetes.yaml", KUBERNETES),
            ] {
                let path = root.join(path);
                fs::create_dir_all(path.parent().unwrap()).unwrap();
                fs::write(path, text).unwrap();
            }
            fs::create_dir_all(root.join("schema/v1")).unwrap();
            for name in SCHEMAS {
                fs::copy(schemas.join(name), root.join("schema/v1").join(name)).unwrap();
            }
            write(&root, "0.5.0", &"a".repeat(40), &root.join("out/v1")).unwrap();
            Self(root)
        }

        pub(crate) fn dir(&self) -> PathBuf {
            self.0.join("out/v1")
        }

        pub(crate) fn problems(&self) -> Vec<Problem> {
            check(
                &self.dir(),
                &Schemas::read(&self.dir().join("schema")).unwrap(),
            )
            .unwrap()
        }

        fn json(&self, file: &str) -> Value {
            read_json(&self.dir().join(file)).unwrap()
        }
    }

    impl Drop for Api {
        fn drop(&mut self) {
            fs::remove_dir_all(&self.0).unwrap_or_default();
        }
    }

    #[test]
    fn publishes_the_declaration_render_and_measurements() {
        let api = Api::new("component");
        assert_eq!(api.problems(), []);
        let doc = api.json("components/web.json");
        let written: Value = read_yaml(&api.0.join("components/web/component.yaml")).unwrap();
        assert_eq!(doc["declaration"], written, "component.yaml as written");

        let file: ComponentFile = serde_json::from_value(doc).unwrap();
        assert_eq!(file.render.kubernetes_version, "1.37.0");
        assert!(
            !file.render.profiles.contains_key(&Profile::MultiNode),
            "unsupported"
        );
        let profile = &file.render.profiles[&Profile::SingleNode];
        let workload = |name: &str| profile.workloads.iter().find(|w| w.name == name).unwrap();
        let web = workload("web");
        assert_eq!((web.replicas, web.namespace.as_str()), (Some(2), "web"));
        assert_eq!(
            serde_json::to_value(&web.containers[0]).unwrap(),
            json!({"name": "web", "requests": {"cpu": "10m", "memory": "64Mi"},
                   "limits": {"cpu": "500m", "memory": "1Gi"}})
        );
        let db = workload("db");
        assert_eq!(db.replicas, Some(1));
        assert_eq!(
            (
                db.volume_claim_templates[0].name.as_str(),
                db.volume_claim_templates[0].storage.as_str()
            ),
            ("data", "1Gi")
        );
        let agent = workload("agent");
        assert_eq!(
            (agent.replicas, agent.namespace.as_str()),
            (None, "kube-system")
        );
        let cache = &profile.persistent_volume_claims[0];
        assert_eq!(
            (
                cache.name.as_str(),
                cache.namespace.as_deref(),
                cache.storage.as_str()
            ),
            ("cache", Some("web"), "5Gi")
        );
        assert_eq!(file.measurements[0].pods[0].memory, "30Mi");
        assert_eq!(file.readmes, ["en", "zh-TW"]);
        assert_eq!(
            fs::read_to_string(api.dir().join("components/web/README.zh-TW.md")).unwrap(),
            "# Web 網頁\n"
        );
    }

    #[test]
    fn publishes_the_template_as_written() {
        let api = Api::new("template");
        assert_eq!(api.problems(), []);
        let doc = api.json("templates/blog.json");
        let written: Value = read_yaml(&api.0.join("templates/blog/template.yaml")).unwrap();
        assert_eq!(doc["template"], written, "template.yaml as written");
        let file: TemplateFile = serde_json::from_value(doc).unwrap();
        assert_eq!(file.manifests, BLOG_MANIFESTS);
        let digest = source_digest(&api.0.join("templates/blog")).unwrap();
        assert_eq!(file.source_digest, digest);
        assert_eq!(file.measurements[0].pods[0].memory, "80Mi");
        assert_eq!(file.readmes, ["en"]);
        assert!(api.dir().join("templates/blog/README.md").exists());
        let index: Index = serde_json::from_value(api.json("catalog.json")).unwrap();
        assert_eq!(
            index.templates,
            [Entry {
                id: "blog".to_owned(),
                source_digest: digest
            }]
        );
        assert_eq!(index.catalog.commit, "a".repeat(40));
        assert!(api.dir().join("schema/template.schema.json").exists());
    }

    #[test]
    fn reads_the_locale_of_a_readme() {
        for (name, locale) in [
            ("README.md", Some("en")),
            ("README.zh-CN.md", Some("zh-CN")),
            ("README..md", None),
            ("README.txt", None),
            ("component.yaml", None),
        ] {
            assert_eq!(readme_locale(name), locale, "{name}");
        }
    }

    #[test]
    fn check_reports_a_broken_file() {
        let api = Api::new("broken");
        for (file, pointer, value) in [
            ("components/web.json", "/sourceDigest", None),
            ("templates/blog.json", "/manifests", None),
            ("templates/blog.json", "/template/replicas", Some(json!(2))),
            (
                "components/web.json",
                "/declaration/replicas",
                Some(json!(2)),
            ),
            ("catalog.json", "/catalog/commit", Some(json!("main"))),
            (
                "components/web.json",
                "/measurements/0/measuredAt",
                Some(json!("not-a-date")),
            ),
        ] {
            let path = api.dir().join(file);
            let text = fs::read_to_string(&path).unwrap();
            let mut doc: Value = serde_json::from_str(&text).unwrap();
            let (parent, key) = pointer.rsplit_once('/').unwrap();
            let object = doc.pointer_mut(parent).unwrap().as_object_mut().unwrap();
            match value {
                Some(value) => object.insert(key.to_owned(), value),
                None => object.remove(key),
            };
            fs::write(&path, doc.to_string()).unwrap();
            let problems = api.problems();
            assert!(!problems.is_empty(), "{file} {pointer}");
            assert!(
                problems
                    .iter()
                    .all(|p| p.file == path && matches!(p.kind, ProblemKind::Schema(_))),
                "{problems:?}"
            );
            fs::write(&path, text).unwrap();
        }
        assert_eq!(api.problems(), []);
    }
}
