//! `zeabur-catalog render`: the objects of every release of every component, on each profile it
//! supports and each Kubernetes version of `config/kubernetes.yaml`, checked with kubeconform.
//!
//! Helm renders the charts. This module only composes the values that the catalog owns, and
//! fetches charts and manifests into a cache.

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use k8s_openapi::apiextensions_apiserver::pkg::apis::apiextensions::v1::{
    CustomResourceDefinition, JSONSchemaProps, JSONSchemaPropsOrArray, JSONSchemaPropsOrBool,
};
use serde::Deserialize;
use serde_json::{Map, Value};
use sha2::{Digest, Sha256};
use zeabur_catalog_format::Catalog;
use zeabur_catalog_format::common::{Mode, Profile};
use zeabur_catalog_format::component::{
    Component, HelmRelease, ManifestRelease, Release, default_accounts,
};
use zeabur_catalog_format::contributions::Contributions;
use zeabur_catalog_format::problem::{FieldPath, Problem, Problems, RenderProblem, Rendering};
use zeabur_catalog_format::values::release_values;

/// `config/kubernetes.yaml`: the versions every profile is rendered against.
#[derive(Deserialize)]
struct Kubernetes {
    versions: Vec<String>,
}

/// The Kubernetes versions of `config/kubernetes.yaml`, oldest first.
pub(crate) fn kube_versions(root: &Path) -> Result<Vec<String>, String> {
    let path = root.join("config/kubernetes.yaml");
    let text = fs::read_to_string(&path).map_err(|e| format!("{}: {e}", path.display()))?;
    let file: Kubernetes =
        serde_saphyr::from_str(&text).map_err(|e| format!("{}: {e}", path.display()))?;
    Ok(file.versions)
}

/// Where pulled charts and downloaded manifests stay between runs.
pub(crate) struct Cache(PathBuf);

impl Cache {
    /// `catalog` in the user's cache directory, the one Go's `os.UserCacheDir` names.
    pub(crate) fn user() -> Result<Self, String> {
        let home = || std::env::var_os("HOME").map(PathBuf::from);
        let base = if cfg!(target_os = "macos") {
            home().map(|h| h.join("Library/Caches"))
        } else {
            std::env::var_os("XDG_CACHE_HOME")
                .map(PathBuf::from)
                .or_else(|| home().map(|h| h.join(".cache")))
        };
        base.map(|b| Self(b.join("catalog")))
            .ok_or_else(|| "no cache directory: set HOME".to_owned())
    }

    /// The entry of `parts`: the SHA-256 of the parts, so different parts never share one.
    fn entry(&self, kind: &str, parts: &[&str]) -> PathBuf {
        self.0
            .join(kind)
            .join(hex(&Sha256::digest(parts.join("\0"))))
    }

    /// `helm` with this cache's own repositories and registry logins, not the user's.
    fn helm(&self) -> Command {
        let mut cmd = Command::new("helm");
        let helm = self.0.join("helm");
        cmd.env("HELM_REPOSITORY_CONFIG", helm.join("repositories.yaml"))
            .env("HELM_REPOSITORY_CACHE", helm.join("repository"))
            .env("HELM_REGISTRY_CONFIG", helm.join("registry.json"));
        cmd
    }

    /// The `.tgz` of the chart of `release`, pulled once. With a `chartDigest`, it fails unless
    /// the chart has that digest.
    fn chart(&self, release: &HelmRelease) -> Result<PathBuf, String> {
        let repo = release.repo.as_deref().unwrap_or_default();
        let dir = self.entry(
            "charts",
            &[
                repo,
                &release.chart,
                &release.version,
                &release.chart_digest,
            ],
        );
        if tgz(&dir).is_none() {
            fs::create_dir_all(&dir).map_err(|e| format!("{}: {e}", dir.display()))?;
            let mut cmd = self.helm();
            cmd.arg("pull");
            if !repo.is_empty() {
                cmd.args([
                    &release.chart,
                    "--version",
                    &release.version,
                    "--repo",
                    repo,
                ]);
            } else if release.chart_digest.is_empty() {
                cmd.args([&release.chart, "--version", &release.version]);
            } else {
                // Helm refuses an OCI chart whose manifest has another digest.
                cmd.arg(format!(
                    "{}:{}@{}",
                    release.chart, release.version, release.chart_digest
                ));
            }
            run(cmd.arg("--destination").arg(&dir))?;
        }
        let chart = tgz(&dir).ok_or_else(|| {
            format!(
                "helm pull {}: expected one chart in {}",
                release.chart,
                dir.display()
            )
        })?;
        if !repo.is_empty() && !release.chart_digest.is_empty() {
            let data = fs::read(&chart).map_err(|e| format!("{}: {e}", chart.display()))?;
            let what = format!("{} {}", release.chart, release.version);
            if let Err(e) = check_digest(&what, &data, &release.chart_digest) {
                fs::remove_dir_all(&dir).map_err(|e| format!("{}: {e}", dir.display()))?;
                return Err(e);
            }
        }
        Ok(chart)
    }

    /// The body of `url`, downloaded once.
    fn download(&self, url: &str) -> Result<Vec<u8>, String> {
        let file = self.entry("manifests", &[url]);
        if let Ok(data) = fs::read(&file) {
            return Ok(data);
        }
        let mut curl = Command::new("curl");
        // --retry retries a timeout, 429 and 5xx with a growing wait.
        curl.args(["-fsSL", "--retry", "4", url]);
        let data = run(&mut curl)?;
        if let Some(dir) = file.parent() {
            fs::create_dir_all(dir).map_err(|e| format!("{}: {e}", dir.display()))?;
        }
        fs::write(&file, &data).map_err(|e| format!("{}: {e}", file.display()))?;
        Ok(data)
    }

    /// The objects of the URLs of `release`, each checked against its `urlDigests` entry.
    fn manifests(&self, release: &ManifestRelease) -> Result<Vec<u8>, String> {
        let mut out = Vec::new();
        for (i, url) in release.resolved_urls().iter().enumerate() {
            let data = self.download(url)?;
            if let Some(want) = release.url_digests.get(i) {
                check_digest(url, &data, want)?;
            }
            out.extend_from_slice(&data);
            out.extend_from_slice(b"\n---\n");
        }
        Ok(out)
    }

    /// The objects of `release` of `component` on `profile` at `kube_version`, with the test
    /// value of each input and what other components contribute.
    pub(crate) fn render(
        &self,
        component: &Component,
        release: &Release,
        profile: Profile,
        kube_version: &str,
        contributions: &Contributions,
        id: &str,
    ) -> Result<Vec<u8>, String> {
        match release {
            Release::Helm(helm) => {
                let chart = self.chart(helm)?;
                let values = release_values(
                    component,
                    helm,
                    profile,
                    contributions.to(id, &helm.name),
                    &test_inputs(component),
                )
                .map_err(|e| e.to_string())?;
                let file = self.entry("values", &[id, &helm.name, profile.as_str()]);
                if let Some(dir) = file.parent() {
                    fs::create_dir_all(dir).map_err(|e| format!("{}: {e}", dir.display()))?;
                }
                // JSON is YAML that every YAML reader takes the same way.
                let json = serde_json::to_vec(&values).map_err(|e| e.to_string())?;
                fs::write(&file, json).map_err(|e| format!("{}: {e}", file.display()))?;
                let mut cmd = self.helm();
                cmd.args(["template", helm.helm_name()])
                    .arg(&chart)
                    .args(["--namespace", &helm.namespace, "--include-crds"])
                    .args(["--kube-version", kube_version, "-f"])
                    .arg(&file);
                run(&mut cmd)
            }
            Release::Manifest(manifest) => self.manifests(manifest),
            Release::Inline(inline) => Ok(inline.manifests.clone().into_bytes()),
            Release::Unknown => Err("a release of an unknown type".to_owned()),
        }
    }
}

/// The one `.tgz` in `dir`.
fn tgz(dir: &Path) -> Option<PathBuf> {
    let found: Vec<PathBuf> = fs::read_dir(dir)
        .ok()?
        .filter_map(|e| e.ok().map(|e| e.path()))
        .filter(|p| p.extension().is_some_and(|x| x == "tgz"))
        .collect();
    match found.as_slice() {
        [one] => Some(one.clone()),
        _ => None,
    }
}

/// The value CI installs each input with: its `testValue`, else its `default`.
#[expect(
    clippy::disallowed_types,
    reason = "an input's value is a JSON value of the type the input declares"
)]
fn test_inputs(component: &Component) -> Map<String, Value> {
    component
        .inputs
        .iter()
        .filter_map(|input| {
            let value = input.test_value.as_ref().or(input.default.as_ref())?;
            Some((input.name.clone(), value.to_json()))
        })
        .collect()
}

fn hex(bytes: &[u8]) -> String {
    bytes
        .iter()
        .flat_map(|b| [b >> 4, b & 0xF])
        .filter_map(|n| char::from_digit(u32::from(n), 16))
        .collect()
}

/// Fails unless `data` has the digest `want` (`sha256:<hex>`).
fn check_digest(what: &str, data: &[u8], want: &str) -> Result<(), String> {
    let got = format!("sha256:{}", hex(&Sha256::digest(data)));
    if got == want {
        Ok(())
    } else {
        Err(format!("{what}: the digest is {got}, not {want}"))
    }
}

/// Runs `cmd` and returns its stdout; a failure carries its stderr.
fn run(cmd: &mut Command) -> Result<Vec<u8>, String> {
    let out = cmd
        .output()
        .map_err(|e| format!("{}: {e}", cmd.get_program().to_string_lossy()))?;
    if out.status.success() {
        return Ok(out.stdout);
    }
    let args: Vec<_> = cmd.get_args().map(|a| a.to_string_lossy()).collect();
    Err(format!(
        "{} {}: {}: {}",
        cmd.get_program().to_string_lossy(),
        args.join(" "),
        out.status,
        String::from_utf8_lossy(&out.stderr).trim()
    ))
}

/// Renders `ids` (every component when empty) into `out/<version>/<profile>/<id>/<release>.yaml`
/// and checks the objects with kubeconform. It returns the problems.
pub(crate) fn render(root: &Path, ids: &[String], out: &Path) -> Result<Vec<Problem>, String> {
    let catalog = Catalog::read(root).map_err(|e| e.to_string())?;
    for id in ids {
        if !catalog.components.contains_key(id) {
            return Err(format!("no component {id}"));
        }
    }
    let versions = kube_versions(root)?;
    let cache = Cache::user()?;
    let contributions = Contributions::of(
        &catalog.components,
        catalog.components.keys().map(String::as_str),
    )
    .map_err(|e| e.to_string())?;
    let schemas = PathBuf::from(format!("{}-schemas", out.display()));
    for dir in [out, schemas.as_path()] {
        if dir.exists() {
            fs::remove_dir_all(dir).map_err(|e| format!("{}: {e}", dir.display()))?;
        }
    }
    let mut problems = Vec::new();
    for (id, component) in &catalog.components {
        if !ids.is_empty() && !ids.contains(id) {
            continue;
        }
        let file = Path::new("components").join(id).join("component.yaml");
        let mut found = Problems::new(&file);
        let accounts = default_accounts(component);
        for release in accounts.iter().chain(&component.releases) {
            let name = release.name().unwrap_or_default();
            for version in &versions {
                for profile in Profile::ALL {
                    if *component.defaults.get(profile) == Mode::Unsupported {
                        continue;
                    }
                    let on = Rendering {
                        release: name.to_owned(),
                        profile,
                        version: version.clone(),
                    };
                    let objects = match cache.render(
                        component,
                        release,
                        profile,
                        version,
                        &contributions,
                        id,
                    ) {
                        Ok(objects) => objects,
                        Err(message) => {
                            found.push(FieldPath::root(), RenderProblem::Failed { on, message });
                            continue;
                        }
                    };
                    let file = out
                        .join(version)
                        .join(profile.as_str())
                        .join(id)
                        .join(format!("{name}.yaml"));
                    if let Some(dir) = file.parent() {
                        fs::create_dir_all(dir).map_err(|e| format!("{}: {e}", dir.display()))?;
                    }
                    fs::write(&file, &objects).map_err(|e| format!("{}: {e}", file.display()))?;
                    if let Err(message) = crd_schemas(&objects, &schemas) {
                        found.push(FieldPath::root(), RenderProblem::CrdSchemas { on, message });
                    }
                }
            }
        }
        problems.extend(found.into_vec());
        crate::output::info(&format!("rendered {id}"));
    }
    for version in &versions {
        let mut cmd = Command::new("kubeconform");
        cmd.args(["-kubernetes-version", version, "-strict", "-summary"])
            .args(["-skip", "CustomResourceDefinition"])
            .args(["-schema-location", "default", "-schema-location"])
            .arg(format!(
                "{}/{{{{.Group}}}}/{{{{.ResourceKind}}}}_{{{{.ResourceAPIVersion}}}}.json",
                schemas.display()
            ))
            .arg(out.join(version));
        let message = match cmd.status() {
            Ok(status) if status.success() => continue,
            Ok(status) => status.to_string(),
            Err(e) => e.to_string(),
        };
        let mut found = Problems::new(&out.join(version));
        found.push(FieldPath::root(), RenderProblem::Kubeconform { message });
        problems.extend(found.into_vec());
    }
    Ok(problems)
}

/// The documents of a YAML stream, without the empty ones.
#[expect(
    clippy::disallowed_types,
    reason = "a rendered object may be of any kind, including ones no type of ours knows"
)]
pub(crate) fn documents(text: &[u8]) -> Result<Vec<Value>, String> {
    let text = std::str::from_utf8(text).map_err(|e| e.to_string())?;
    let docs: Vec<Value> = serde_saphyr::from_multiple(text).map_err(|e| e.to_string())?;
    Ok(docs.into_iter().filter(|d| !d.is_null()).collect())
}

/// Writes a kubeconform schema for each version of each CRD in `objects`, at
/// `dir/<group>/<kind>_<version>.json`, so custom resources are checked against the CRDs they
/// install with.
#[expect(
    clippy::disallowed_types,
    clippy::disallowed_methods,
    reason = "only its kind tells whether a rendered object is a CRD"
)]
fn crd_schemas(objects: &[u8], dir: &Path) -> Result<(), String> {
    for doc in documents(objects)? {
        if doc.get("kind").and_then(Value::as_str) != Some("CustomResourceDefinition") {
            continue;
        }
        let crd: CustomResourceDefinition =
            serde_json::from_value(doc).map_err(|e| format!("a CRD: {e}"))?;
        let spec = crd.spec;
        for version in spec.versions {
            let Some(mut schema) = version.schema.and_then(|s| s.open_api_v3_schema) else {
                continue;
            };
            for property in schema.properties.iter_mut().flat_map(|p| p.values_mut()) {
                strict(property);
            }
            let file = dir.join(&spec.group).join(format!(
                "{}_{}.json",
                spec.names.kind.to_lowercase(),
                version.name
            ));
            if let Some(parent) = file.parent() {
                fs::create_dir_all(parent).map_err(|e| format!("{}: {e}", parent.display()))?;
            }
            let json = serde_json::to_vec(&schema).map_err(|e| e.to_string())?;
            fs::write(&file, json).map_err(|e| format!("{}: {e}", file.display()))?;
        }
    }
    Ok(())
}

/// Refuses unknown fields as the API server does, and writes Kubernetes' int-or-string as JSON
/// Schema.
fn strict(schema: &mut JSONSchemaProps) {
    if schema.x_kubernetes_int_or_string == Some(true) {
        schema.type_ = None;
        schema.one_of = Some(vec![
            JSONSchemaProps {
                type_: Some("string".to_owned()),
                ..JSONSchemaProps::default()
            },
            JSONSchemaProps {
                type_: Some("integer".to_owned()),
                ..JSONSchemaProps::default()
            },
        ]);
    }
    if schema.properties.as_ref().is_some_and(|p| !p.is_empty())
        && schema.additional_properties.is_none()
        && schema.x_kubernetes_preserve_unknown_fields != Some(true)
    {
        schema.additional_properties = Some(JSONSchemaPropsOrBool::Bool(false));
    }
    for property in schema.properties.iter_mut().flat_map(|p| p.values_mut()) {
        strict(property);
    }
    match &mut schema.items {
        Some(JSONSchemaPropsOrArray::Schema(item)) => strict(item),
        Some(JSONSchemaPropsOrArray::Schemas(items)) => items.iter_mut().for_each(strict),
        None => {}
    }
    if let Some(JSONSchemaPropsOrBool::Schema(extra)) = &mut schema.additional_properties {
        strict(extra);
    }
    for list in [&mut schema.all_of, &mut schema.any_of, &mut schema.one_of]
        .into_iter()
        .flatten()
    {
        list.iter_mut().for_each(strict);
    }
}

#[cfg(test)]
mod tests {
    #![expect(
        clippy::disallowed_methods,
        reason = "the cases compare written schemas and documents as JSON"
    )]

    use serde_json::json;

    use super::*;

    #[test]
    fn a_crd_schema_refuses_unknown_fields_and_takes_int_or_string() {
        let mut schema: JSONSchemaProps = serde_json::from_value(json!({
            "type": "object",
            "properties": {
                "port": {"x-kubernetes-int-or-string": true},
                "list": {"type": "array", "items": {"type": "object", "properties": {"a": {"type": "string"}}}},
                "open": {
                    "type": "object",
                    "x-kubernetes-preserve-unknown-fields": true,
                    "properties": {"a": {"type": "string"}},
                },
            },
        }))
        .unwrap();
        for property in schema.properties.iter_mut().flat_map(|p| p.values_mut()) {
            strict(property);
        }
        let got = serde_json::to_value(&schema).unwrap();
        assert_eq!(
            got["properties"]["port"]["oneOf"],
            json!([{"type": "string"}, {"type": "integer"}])
        );
        assert_eq!(got["properties"]["port"].get("type"), None);
        assert_eq!(
            got["properties"]["list"]["items"]["additionalProperties"],
            json!(false)
        );
        assert_eq!(got["properties"]["open"].get("additionalProperties"), None);
        // The top level stays open: kubeconform checks apiVersion, kind and metadata there.
        assert_eq!(got.get("additionalProperties"), None);
    }

    #[test]
    fn a_stream_has_its_documents_without_the_empty_ones() {
        let docs = documents(b"# Source: a\n---\nkind: A\n---\n---\nkind: B\n").unwrap();
        assert_eq!(docs, [json!({"kind": "A"}), json!({"kind": "B"})]);
    }

    #[test]
    fn a_digest_must_match() {
        let empty = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855";
        assert_eq!(check_digest("x", b"", empty), Ok(()));
        assert!(check_digest("x", b"a", empty).is_err());
    }
}
