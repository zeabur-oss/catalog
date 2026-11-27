//! What the API server refuses to update: each release applied for real from one render, then
//! sent from another with `--dry-run=server`. The two profiles are compared in both directions,
//! and the base's render of each profile with this one. The dry run already sent every render to
//! an empty API server, so a refusal here comes from updating one render to the other.

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::Duration;

use zeabur_catalog_format::Catalog;
use zeabur_catalog_format::common::{Mode, Profile};
use zeabur_catalog_format::component::default_accounts;
use zeabur_catalog_format::problem::{DryRunProblem, FieldPath, Problem};

use super::server::{Kubectl, tail};
use super::{Doc, by_namespace, docs, minor};

/// A release the API server refused to update, and what kubectl printed.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Refusal {
    /// The component.
    pub(crate) component: String,
    /// The release.
    pub(crate) release: String,
    /// The update that was refused.
    pub(crate) update: Update,
    /// What kubectl printed.
    pub(crate) output: String,
}

/// An update of a release.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Update {
    /// From one profile to the other.
    Switch(Profile, Profile),
    /// From the base's render on a profile.
    Upgrade(Profile),
}

/// A release of a component: its name and namespace.
type ReleaseName<'a> = (&'a str, &'a str);

/// Compares the renders in `dir` with each other and with the base's, and returns a problem for
/// each refusal that [`judge`] keeps.
pub(crate) fn check(
    root: &Path,
    catalog: &Catalog,
    dir: &Path,
    kubectl: &Kubectl,
) -> Result<Vec<Problem>, String> {
    let base = render_base(root)?;
    let base_dir = match &base {
        None => None,
        Some((_, rendered)) => {
            let version = dir
                .file_name()
                .map_or_else(String::new, |v| v.to_string_lossy().into_owned());
            let found: Vec<PathBuf> = fs::read_dir(rendered)
                .map_err(|e| format!("{}: {e}", rendered.display()))?
                .filter_map(|e| Some(e.ok()?.path()))
                .filter(|p| {
                    p.file_name()
                        .is_some_and(|n| minor(&n.to_string_lossy()) == minor(&version))
                })
                .collect();
            match found.as_slice() {
                [one] => Some(one.clone()),
                _ => {
                    return Err(format!(
                        "the base renders no Kubernetes {}",
                        minor(&version)
                    ));
                }
            }
        }
    };
    let mut found = Vec::new();
    for (id, component) in &catalog.components {
        let accounts = default_accounts(component);
        let releases: Vec<ReleaseName<'_>> = accounts
            .iter()
            .chain(&component.releases)
            .filter_map(|r| Some((r.name()?, r.namespace()?)))
            .collect();
        let supported = |p: Profile| *component.defaults.get(p) != Mode::Unsupported;
        let [a, b] = Profile::ALL;
        let mut compare = |update: Update, from: &Path, to: &Path| -> Result<(), String> {
            for (release, output) in refused(id, &releases, from, to, kubectl)? {
                found.push(Refusal {
                    component: id.clone(),
                    release,
                    update,
                    output,
                });
            }
            Ok(())
        };
        if supported(a) && supported(b) {
            let (pa, pb) = (dir.join(a.as_str()), dir.join(b.as_str()));
            compare(Update::Switch(a, b), &pa, &pb)?;
            compare(Update::Switch(b, a), &pb, &pa)?;
        }
        if let Some(base_dir) = &base_dir {
            for p in Profile::ALL.into_iter().filter(|p| supported(*p)) {
                compare(
                    Update::Upgrade(p),
                    &base_dir.join(p.as_str()),
                    &dir.join(p.as_str()),
                )?;
            }
        }
    }
    let major = match &base {
        Some((sha, _)) => adds_major_change_file(root, sha)?,
        None => false,
    };
    Ok(judge(found, major))
}

/// A problem for each refusal. An upgrade refusal passes when the pull request adds a `major`
/// change file, which says how to migrate; a profile switch never does.
pub(crate) fn judge(found: Vec<Refusal>, major: bool) -> Vec<Problem> {
    let mut out = Vec::new();
    for refusal in found {
        let kind = match refusal.update {
            Update::Upgrade(_) if major => {
                crate::output::info(&format!(
                    "intended break: {} release {}",
                    refusal.component, refusal.release
                ));
                continue;
            }
            Update::Upgrade(profile) => DryRunProblem::Upgrade {
                release: refusal.release,
                profile,
                output: refusal.output,
            },
            Update::Switch(from, to) => DryRunProblem::ProfileSwitch {
                release: refusal.release,
                from,
                to,
                output: refusal.output,
            },
        };
        out.push(Problem {
            file: Path::new("components")
                .join(&refusal.component)
                .join("component.yaml"),
            at: FieldPath::root(),
            kind: kind.into(),
        });
    }
    out
}

/// Renders the merge-base of `HEAD` and the base branch, `$CATALOG_BASE` or `origin/main`, with
/// its own `zeabur-catalog render` into `out/render-base`. It returns the merge-base and the
/// directory, or nothing when `HEAD` is the merge-base or there is none.
fn render_base(root: &Path) -> Result<Option<(String, PathBuf)>, String> {
    let reference = std::env::var("CATALOG_BASE").unwrap_or_else(|_| "origin/main".to_owned());
    if !is_ref_name(&reference) {
        return Err(format!(
            "CATALOG_BASE {reference:?} is not a branch or commit name"
        ));
    }
    let Ok(sha) = git(root, &["merge-base", "HEAD", &reference]) else {
        crate::output::info(&format!(
            "no base to upgrade from: no merge-base with {reference}"
        ));
        return Ok(None);
    };
    if sha == git(root, &["rev-parse", "HEAD"])? {
        return Ok(None);
    }
    let src = std::path::absolute(root.join("out/base-src")).map_err(|e| e.to_string())?;
    let out = std::path::absolute(root.join("out/render-base")).map_err(|e| e.to_string())?;
    if src.exists() {
        fs::remove_dir_all(&src).map_err(|e| format!("{}: {e}", src.display()))?;
    }
    fs::create_dir_all(&src).map_err(|e| format!("{}: {e}", src.display()))?;
    let archive = Command::new("git")
        .args(["archive", &sha])
        .current_dir(root)
        .output()
        .map_err(|e| format!("git archive: {e}"))?;
    if !archive.status.success() {
        return Err(format!(
            "git archive {sha}: {}",
            String::from_utf8_lossy(&archive.stderr).trim()
        ));
    }
    let mut extract = Command::new("tar");
    extract.arg("-x").arg("-C").arg(&src);
    run_with_input(&mut extract, &archive.stdout)?;
    crate::output::info(&format!("rendering the base {sha}"));
    let rendered = Command::new("cargo")
        .args([
            "run",
            "--locked",
            "-q",
            "-p",
            "zeabur-catalog",
            "--",
            "render",
            "--out",
        ])
        .arg(&out)
        .current_dir(&src)
        .output()
        .map_err(|e| format!("cargo: {e}"))?;
    let removed = fs::remove_dir_all(&src);
    if !rendered.status.success() {
        let log = String::from_utf8_lossy(&rendered.stderr);
        return Err(format!("render {sha}: {}", tail(&log, 20)));
    }
    removed.map_err(|e| format!("{}: {e}", src.display()))?;
    Ok(Some((sha, out)))
}

fn run_with_input(cmd: &mut Command, input: &[u8]) -> Result<(), String> {
    use std::io::Write as _;
    let mut child = cmd
        .stdin(std::process::Stdio::piped())
        .spawn()
        .map_err(|e| format!("{}: {e}", cmd.get_program().display()))?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin.write_all(input).map_err(|e| e.to_string())?;
    }
    let status = child.wait().map_err(|e| e.to_string())?;
    if status.success() {
        Ok(())
    } else {
        Err(format!("{}: {status}", cmd.get_program().display()))
    }
}

/// Whether `name` is a plain ref or commit name that git can't read as an option: ASCII letters,
/// digits, `.`, `_`, `-` and `/`, not starting with `-`.
fn is_ref_name(name: &str) -> bool {
    !name.is_empty()
        && !name.starts_with('-')
        && name
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || matches!(b, b'.' | b'_' | b'-' | b'/'))
}

/// `git <args>` in `dir`, its output trimmed.
fn git(dir: &Path, args: &[&str]) -> Result<String, String> {
    let out = Command::new("git")
        .args(args)
        .current_dir(dir)
        .output()
        .map_err(|e| format!("git: {e}"))?;
    if !out.status.success() {
        return Err(format!(
            "git {}: {}",
            args.join(" "),
            String::from_utf8_lossy(&out.stderr).trim()
        ));
    }
    Ok(String::from_utf8_lossy(&out.stdout).trim().to_owned())
}

/// Whether `HEAD` adds a change file under `.changeset/` since `base` whose frontmatter says
/// `default: major`.
pub(crate) fn adds_major_change_file(dir: &Path, base: &str) -> Result<bool, String> {
    let names = git(
        dir,
        &[
            "diff",
            "--name-only",
            "--diff-filter=A",
            base,
            "HEAD",
            "--",
            ".changeset/",
        ],
    )?;
    for name in names.lines().filter(|n| !n.is_empty()) {
        let path = dir.join(name);
        let text = fs::read_to_string(&path).map_err(|e| format!("{}: {e}", path.display()))?;
        let mut lines = text.lines();
        if lines.next() != Some("---") {
            continue;
        }
        if lines
            .take_while(|l| *l != "---")
            .any(|l| l.trim() == "default: major")
        {
            return Ok(true);
        }
    }
    Ok(false)
}

/// Applies each release of the component `id` under `from`, then sends the same release under
/// `to` with `--dry-run=server`, and returns kubectl's output for each release the API server
/// refuses to update. A release missing under `from` is new and skipped; one that renders the
/// same is too. When the CRDs differ, `from`'s are installed before `from`'s objects and `to`'s
/// after them, as in an upgrade. An object that does not apply against its own CRDs is an
/// error, not a refusal. The applied objects are removed again.
pub(crate) fn refused(
    id: &str,
    releases: &[ReleaseName<'_>],
    from: &Path,
    to: &Path,
    kubectl: &Kubectl,
) -> Result<Vec<(String, String)>, String> {
    let mut applied = Vec::new();
    let result = refused_into(id, releases, from, to, kubectl, &mut applied);
    for (namespace, text) in applied.iter().rev() {
        let n = ["-n", namespace.as_str()];
        let delete = [
            &["delete", "--wait=false", "--ignore-not-found"][..],
            &n,
            &["-f", "-"],
        ];
        // The API server has no controller to clear finalizers, so this clears them.
        let patch = [
            &[
                "patch",
                "--type=merge",
                "-p",
                r#"{"metadata":{"finalizers":null}}"#,
            ][..],
            &n,
            &["-f", "-"],
        ];
        for args in [delete.concat(), patch.concat()] {
            // A leftover object only slows the next comparison, so a failure here is not one.
            let _done: Result<String, _> = kubectl.run(text.as_bytes(), &args);
        }
    }
    result
}

fn refused_into(
    id: &str,
    releases: &[ReleaseName<'_>],
    from: &Path,
    to: &Path,
    kubectl: &Kubectl,
    applied: &mut Vec<(String, String)>,
) -> Result<Vec<(String, String)>, String> {
    let mut found: Vec<(String, String)> = Vec::new();
    for &(name, namespace) in releases {
        let file = format!("{name}.yaml");
        let before = match fs::read(from.join(id).join(&file)) {
            Ok(data) => data,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => continue,
            Err(e) => return Err(format!("{}: {e}", from.join(id).join(&file).display())),
        };
        let path = to.join(id).join(&file);
        let after = fs::read(&path).map_err(|e| format!("{}: {e}", path.display()))?;
        if before == after {
            continue;
        }
        let old = docs(&before, namespace).map_err(|e| format!("{name}: {e}"))?;
        let new = docs(&after, namespace).map_err(|e| format!("{name}: {e}"))?;
        let (old_crds, new_crds) = (Crds::of(&old), Crds::of(&new));
        let changed = old_crds.text != new_crds.text;
        if changed {
            old_crds
                .reset(&new_crds.names, kubectl)
                .map_err(|e| format!("{name}: base CRDs: {e}"))?;
        }
        for (ns, text) in updatable(&old) {
            let namespace = namespace_object(&ns);
            kubectl
                .text(namespace.as_bytes(), &["apply", "-f", "-"])
                .map_err(|e| format!("namespace {ns}: {e}"))?;
            applied.push((ns.clone(), text.clone()));
            kubectl
                .text(
                    text.as_bytes(),
                    &[
                        "apply",
                        "--server-side",
                        "--force-conflicts",
                        "--field-manager=catalog",
                        "-n",
                        &ns,
                        "-f",
                        "-",
                    ],
                )
                .map_err(|e| format!("apply {name}: {e}"))?;
        }
        if changed {
            new_crds
                .install(kubectl)
                .map_err(|e| format!("{name}: new CRDs: {e}"))?;
        }
        for (ns, text) in updatable(&new) {
            if let Some(output) = dry_run_update(&ns, &text, changed, kubectl) {
                match found.iter_mut().find(|(r, _)| r == name) {
                    Some((_, all)) => all.push_str(&output),
                    None => found.push((name.to_owned(), output)),
                }
            }
        }
    }
    Ok(found)
}

/// The CRDs of a rendered release.
struct Crds {
    names: Vec<String>,
    text: String,
}

impl Crds {
    fn of(docs: &[Doc]) -> Self {
        let crds = docs.iter().filter(|d| d.kind == "CustomResourceDefinition");
        Self {
            names: crds.clone().map(|d| d.name.clone()).collect(),
            text: super::stream(crds.map(|d| d.text.as_str())),
        }
    }

    /// Deletes the CRDs under their names and under `other`, with their objects, and installs
    /// these, so none is left from the version before.
    fn reset(&self, other: &[String], kubectl: &Kubectl) -> Result<(), String> {
        let gone: Vec<&str> = self.names.iter().chain(other).map(String::as_str).collect();
        if !gone.is_empty() {
            let args = [
                &["delete", "crd", "--ignore-not-found", "--timeout=2m"][..],
                &gone,
            ]
            .concat();
            kubectl.text(&[], &args)?;
        }
        self.install(kubectl)
    }

    /// Applies the CRDs over the ones in the API server, as an upgrade does, and keeps the
    /// objects they hold.
    fn install(&self, kubectl: &Kubectl) -> Result<(), String> {
        if self.names.is_empty() {
            return Ok(());
        }
        kubectl.text(
            self.text.as_bytes(),
            &[
                "apply",
                "--server-side",
                "--force-conflicts",
                "--field-manager=catalog",
                "-f",
                "-",
            ],
        )?;
        let crds: Vec<String> = self.names.iter().map(|n| format!("crd/{n}")).collect();
        let args = [
            &["wait", "--for=condition=Established", "--timeout=2m"][..],
            &crds.iter().map(String::as_str).collect::<Vec<_>>(),
        ]
        .concat();
        kubectl.text(&[], &args).map(drop)
    }
}

/// Sends the new objects of one namespace over the stored ones, and returns what kubectl printed
/// when the API server refuses them. The API server serves an updated CRD's schema a moment after
/// the update, so with new CRDs a "not declared in schema" error is tried again for a few seconds
/// before it counts.
fn dry_run_update(
    namespace: &str,
    text: &str,
    new_crds: bool,
    kubectl: &Kubectl,
) -> Option<String> {
    let args = [
        "apply",
        "--server-side",
        "--force-conflicts",
        "--dry-run=server",
        "--field-manager=catalog",
        "-n",
        namespace,
        "-f",
        "-",
    ];
    let mut last = None;
    for _ in 0..10 {
        match kubectl.run(text.as_bytes(), &args) {
            Ok(_) => return None,
            Err(failed) if new_crds && failed.stderr.contains("not declared in schema") => {
                last = Some(failed.stderr);
                std::thread::sleep(Duration::from_secs(1));
            }
            Err(failed) => return Some(failed.stderr),
        }
    }
    last
}

/// A rendered release's objects by namespace, without what an upgrade never updates in place:
/// CRDs and Namespaces (the dry run installed them), webhook configurations and `APIService`s
/// (nothing serves them here), and Helm hooks (Helm creates them again on every upgrade).
fn updatable(docs: &[Doc]) -> std::collections::BTreeMap<String, String> {
    by_namespace(docs.iter().filter(|d| {
        !d.hook
            && !matches!(
                d.kind.as_str(),
                "CustomResourceDefinition"
                    | "Namespace"
                    | "ValidatingWebhookConfiguration"
                    | "MutatingWebhookConfiguration"
                    | "APIService"
            )
    }))
}

/// The `Namespace` named `name`, as JSON.
fn namespace_object(name: &str) -> String {
    serde_json::json!({"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": name}})
        .to_string()
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::path::{Path, PathBuf};
    use std::process::Command;

    use zeabur_catalog_format::common::Profile;

    use super::super::server::ApiServer;
    use super::{Refusal, Update, adds_major_change_file, judge, refused};

    const TEST: &str = concat!(env!("CARGO_MANIFEST_DIR"), "/../../out/test/refusals");

    /// Release db of component x/y: a StatefulSet whose claim size is SIZE, which Kubernetes
    /// cannot change in place; a Deployment whose image is IMAGE, which it can; and a Helm hook
    /// Job with image IMAGE, whose pod template cannot change but which Helm creates again on
    /// every upgrade.
    const DB: &str = "apiVersion: apps/v1
kind: StatefulSet
metadata: {name: db}
spec:
  serviceName: db
  selector: {matchLabels: {app: db}}
  template:
    metadata: {labels: {app: db}}
    spec:
      containers: [{name: db, image: \"postgres:IMAGE\"}]
  volumeClaimTemplates:
    - metadata: {name: data}
      spec:
        accessModes: [ReadWriteOnce]
        resources: {requests: {storage: SIZE}}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec:
      containers: [{name: web, image: \"nginx:IMAGE\"}]
---
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate
  annotations: {helm.sh/hook: pre-upgrade}
spec:
  template:
    spec:
      restartPolicy: Never
      containers: [{name: migrate, image: \"postgres:IMAGE\"}]
";

    /// A CRD of kind Widget with one spec field, FIELD, and a Widget that sets it to VALUE.
    const WIDGETS: &str = "apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata: {name: widgets.example.test}
spec:
  group: example.test
  scope: Namespaced
  names: {kind: Widget, plural: widgets, singular: widget}
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                FIELD: {type: string RULE}
---
apiVersion: example.test/v1
kind: Widget
metadata: {name: one}
spec: {SET: VALUE}
";

    /// Writes `text` as release `release` of component `id` into a new render directory `name`.
    fn write(name: &str, id: &str, release: &str, text: &str) -> PathBuf {
        let dir = Path::new(TEST).join(name);
        fs::create_dir_all(dir.join(id)).unwrap();
        fs::write(dir.join(id).join(format!("{release}.yaml")), text).unwrap();
        dir
    }

    fn db(name: &str, image: &str, size: &str) -> PathBuf {
        let text = DB.replace("IMAGE", image).replace("SIZE", size);
        write(name, "x-y", "db", &text)
    }

    fn widgets(name: &str, field: &str, set: &str, value: &str, rule: bool) -> PathBuf {
        let rule = if rule {
            ", x-kubernetes-validations: [{rule: self == oldSelf, message: fixed is immutable}]"
        } else {
            ""
        };
        let text = WIDGETS
            .replace("FIELD", field)
            .replace("RULE", rule)
            .replace("SET", set)
            .replace("VALUE", value);
        write(name, "x-z", "w", &text)
    }

    /// An API server, or nothing outside the dev shell.
    fn server(name: &str) -> Option<ApiServer> {
        let found = Command::new("kube-apiserver").arg("--version").output();
        if !found.is_ok_and(|o| o.status.success()) {
            return None;
        }
        Some(ApiServer::start(&Path::new(TEST).join(name)).unwrap())
    }

    /// The API server refuses a StatefulSet whose `volumeClaimTemplates` change, and takes a
    /// render that changes only what updates in place or is a Helm hook, and one that renders
    /// the same.
    #[test]
    fn an_update_the_api_server_refuses_names_its_object() {
        let Some(server) = server("apiserver-db") else {
            return;
        };
        let k = &server.kubectl;
        let releases = [("db", "db")];
        let installed = db("db-17", "17", "1Gi");
        let found = refused("x-y", &releases, &installed, &db("db-18-2", "18", "2Gi"), k).unwrap();
        let [(release, output)] = found.as_slice() else {
            panic!("want one refusal, got {found:?}");
        };
        assert_eq!(release, "db");
        assert!(
            output.contains(r#"StatefulSet "db" is invalid"#),
            "{output}"
        );
        assert!(!output.contains("Deployment"), "{output}");
        let found = refused("x-y", &releases, &installed, &db("db-18-1", "18", "1Gi"), k).unwrap();
        assert!(found.is_empty(), "{found:?}");
        let found = refused("x-y", &releases, &installed, &installed, k).unwrap();
        assert!(found.is_empty(), "{found:?}");
    }

    /// An upgrade starts from the base's CRDs with the base's objects stored, then applies the
    /// new CRDs and sends the new objects over them.
    #[test]
    fn an_upgrade_applies_the_new_crds_over_the_stored_objects() {
        let Some(server) = server("apiserver-crds") else {
            return;
        };
        let k = &server.kubectl;
        let releases = [("w", "w")];
        // A CRD that drops a field and an object that stops setting it pass.
        let from = widgets("planner", "planner", "planner", "a", false);
        let to = widgets("railpack", "railpack", "railpack", "a", false);
        let found = refused("x-z", &releases, &from, &to, k).unwrap();
        assert!(found.is_empty(), "{found:?}");
        // Only the new CRD has the rule, and the base's object is still stored when it arrives.
        let from = widgets("fixed-a", "fixed", "fixed", "a", false);
        let to = widgets("fixed-b", "fixed", "fixed", "b", true);
        let found = refused("x-z", &releases, &from, &to, k).unwrap();
        assert!(
            found
                .iter()
                .any(|(r, o)| r == "w" && o.contains("fixed is immutable")),
            "{found:?}"
        );
        // A base object that does not apply against its own CRD is an error.
        let broken = widgets("broken", "planner", "other", "a", false);
        assert!(refused("x-z", &releases, &broken, &to, k).is_err());
    }

    /// An upgrade refusal passes only when the pull request adds a `major` change file. A
    /// profile switch never passes.
    #[test]
    fn only_an_upgrade_with_a_major_change_file_passes() {
        let output = r#"The StatefulSet "zot" is invalid: spec: Forbidden"#.to_owned();
        let refusal = |update| Refusal {
            component: "zeabur-builder".to_owned(),
            release: "zot".to_owned(),
            update,
            output: output.clone(),
        };
        let upgrade = refusal(Update::Upgrade(Profile::MultiNode));
        let switch = refusal(Update::Switch(Profile::SingleNode, Profile::MultiNode));
        let found = judge(vec![upgrade.clone()], false);
        assert_eq!(found.len(), 1);
        assert!(found[0].to_string().contains(r#"StatefulSet "zot""#));
        assert_eq!(judge(vec![upgrade], true).len(), 0);
        let found = judge(vec![switch], true);
        assert_eq!(found.len(), 1);
        assert!(
            found[0]
                .to_string()
                .contains("from single-node to multi-node")
        );
    }

    /// A repository with one commit that has a `major` change file, and the commit.
    fn repo(name: &str) -> (PathBuf, String) {
        let dir = Path::new(TEST).join(name);
        if dir.exists() {
            fs::remove_dir_all(&dir).unwrap();
        }
        fs::create_dir_all(dir.join(".changeset")).unwrap();
        fs::write(
            dir.join(".changeset/old.md"),
            "---\ndefault: major\n---\n\n# Old\n",
        )
        .unwrap();
        git(&dir, &["init", "-q"]);
        commit(&dir, "base");
        let base = git(&dir, &["rev-parse", "HEAD"]);
        (dir, base)
    }

    #[test]
    fn a_base_that_reads_as_an_option_is_refused() {
        assert!(super::is_ref_name("origin/main"));
        assert!(super::is_ref_name("56e427a7"));
        assert!(!super::is_ref_name("--output=x"));
        assert!(!super::is_ref_name("main;rm"));
        assert!(!super::is_ref_name(""));
    }

    fn git(dir: &Path, args: &[&str]) -> String {
        let out = Command::new("git")
            .args([
                "-c",
                "user.name=t",
                "-c",
                "user.email=t@t",
                "-c",
                "commit.gpgsign=false",
            ])
            .args(args)
            .current_dir(dir)
            .output()
            .unwrap();
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        String::from_utf8_lossy(&out.stdout).trim().to_owned()
    }

    fn commit(dir: &Path, message: &str) {
        git(dir, &["add", "."]);
        git(dir, &["commit", "-q", "-m", message]);
    }

    fn add_change(dir: &Path, name: &str, level: &str) {
        let text = format!(
            "---\ndefault: {level}\n---\n\n# Change\n\nSteps with default: major in the text.\n"
        );
        fs::write(dir.join(".changeset").join(name), text).unwrap();
        commit(dir, name);
    }

    #[test]
    fn a_major_change_file_counts_only_when_the_pull_request_adds_it() {
        let (dir, base) = repo("major-added");
        add_change(&dir, "new.md", "major");
        assert!(adds_major_change_file(&dir, &base).unwrap());
        let (dir, base) = repo("minor-added");
        add_change(&dir, "a.md", "minor");
        add_change(&dir, "b.md", "patch");
        assert!(!adds_major_change_file(&dir, &base).unwrap());
        let (dir, base) = repo("none-added");
        assert!(!adds_major_change_file(&dir, &base).unwrap());
    }
}
