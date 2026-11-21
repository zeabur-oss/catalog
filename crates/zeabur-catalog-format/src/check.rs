//! Strict reading for the catalog's own check.
//!
//! Readers of the API take the types as they are: they ignore a field they do not know and read
//! an unknown value as `Unknown` [releases]. The files authors write are held to more: here a
//! field or value the types do not know is an error, named by its path, such as
//! `health[0].conditon`.
//!
//! [releases]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/releases.mdx

use std::path::Path;

use serde::de::DeserializeOwned;
use serde_json::Value;

use crate::api::{MeasuredDistribution, MeasurementFile, RunResult};
use crate::common::{Distribution, Generate, Mode, PerProfile, PodSecurity, WorkloadKind};
use crate::component::{Component, HelmRelease, InlineRelease, ManifestRelease, Release};
use crate::select::{LeftOut, SelectCase, Warning};
use crate::template::Template;
use crate::{ReadError, read_text};

/// A file type the check reads strictly.
pub trait Strict: DeserializeOwned {
    /// Adds the path of each value the types read as `Unknown`, and of each unknown field inside
    /// an internally tagged part of `raw`, which `serde_ignored` cannot see.
    fn problems(&self, raw: &Value, problems: &mut Vec<String>);
}

/// Reads `text` as `T`, failing on every field and value `T` does not know.
///
/// # Errors
///
/// The text is no valid `T`, or it has fields or values `T` does not know: one message each.
pub fn from_yaml<T: Strict>(text: &str) -> Result<T, Vec<String>> {
    let raw: Value = serde_saphyr::from_str(text).map_err(|e| vec![e.to_string()])?;
    let mut problems = Vec::new();
    let Some(read) = decode::<T>(&raw, "", &[], &mut problems) else {
        return Err(problems);
    };
    read.problems(&raw, &mut problems);
    if problems.is_empty() {
        Ok(read)
    } else {
        Err(problems)
    }
}

/// Reads the file at `path` strictly.
///
/// # Errors
///
/// It cannot be read, is no valid `T`, or has fields or values `T` does not know.
pub fn read<T: Strict>(path: &Path) -> Result<T, ReadError> {
    from_yaml(&read_text(path)?).map_err(|problems| ReadError::Format {
        path: path.to_owned(),
        message: problems.join("; "),
    })
}

/// Decodes `raw`, adding each ignored field below `at` except `skip` to `problems`. `None` with
/// the error in `problems` when it does not decode.
fn decode<T: DeserializeOwned>(
    raw: &Value,
    at: &str,
    skip: &[&str],
    problems: &mut Vec<String>,
) -> Option<T> {
    let mut ignored = Vec::new();
    let read = serde_ignored::deserialize(raw.clone(), |path| ignored.push(render(&path)));
    for path in ignored {
        if !skip.contains(&path.as_str()) {
            problems.push(format!("unknown field {}", join(at, &path)));
        }
    }
    match read {
        Ok(read) => Some(read),
        Err(e) => {
            problems.push(format!("{}: {e}", if at.is_empty() { "." } else { at }));
            None
        }
    }
}

fn render(path: &serde_ignored::Path<'_>) -> String {
    match path {
        serde_ignored::Path::Root => String::new(),
        serde_ignored::Path::Seq { parent, index } => format!("{}[{index}]", render(parent)),
        serde_ignored::Path::Map { parent, key } => join(&render(parent), key),
        serde_ignored::Path::Some { parent }
        | serde_ignored::Path::NewtypeStruct { parent }
        | serde_ignored::Path::NewtypeVariant { parent } => render(parent),
    }
}

fn join(at: &str, key: &str) -> String {
    match (at.is_empty(), key.is_empty()) {
        (true, _) => key.to_owned(),
        (false, true) => at.to_owned(),
        (false, false) if key.starts_with('[') => format!("{at}{key}"),
        (false, false) => format!("{at}.{key}"),
    }
}

fn unknown(problems: &mut Vec<String>, at: &str) {
    problems.push(format!("unknown value at {at}"));
}

fn items<'a>(raw: &'a Value, key: &str) -> impl Iterator<Item = &'a Value> {
    raw.get(key).and_then(Value::as_array).into_iter().flatten()
}

/// Checks the keys of a tagged part that has no struct of its own.
fn keys(raw: &Value, at: &str, allowed: &[&str], problems: &mut Vec<String>) {
    for key in raw.as_object().into_iter().flat_map(|m| m.keys()) {
        if !allowed.contains(&key.as_str()) {
            problems.push(format!("unknown field {at}.{key}"));
        }
    }
}

fn modes(defaults: PerProfile<Mode>, at: &str, problems: &mut Vec<String>) {
    for (profile, mode) in [
        ("single-node", defaults.single_node),
        ("multi-node", defaults.multi_node),
    ] {
        if mode == Mode::Unknown {
            unknown(problems, &format!("{at}defaults.{profile}"));
        }
    }
}

fn distributions(list: &[Distribution], at: &str, problems: &mut Vec<String>) {
    for (i, d) in list.iter().enumerate() {
        if *d == Distribution::Unknown {
            unknown(problems, &format!("{at}[{i}]"));
        }
    }
}

fn generate(generate: Option<Generate>, raw: &Value, at: &str, problems: &mut Vec<String>) {
    match generate {
        Some(Generate::Unknown) => unknown(problems, &format!("{at}.type")),
        Some(Generate::Random { .. }) => keys(raw, at, &["type", "bytes", "encoding"], problems),
        Some(Generate::ClusterIp) => keys(raw, at, &["type"], problems),
        None => {}
    }
}

impl Strict for Component {
    fn problems(&self, raw: &Value, problems: &mut Vec<String>) {
        modes(self.defaults, "", problems);
        distributions(&self.distributions, "distributions", problems);
        if self.security.pod_security == PodSecurity::Unknown {
            unknown(problems, "security.podSecurity");
        }
        for (i, (release, raw)) in self.releases.iter().zip(items(raw, "releases")).enumerate() {
            let at = format!("releases[{i}]");
            let skip = &["type"];
            match release {
                Release::Helm(_) => drop(decode::<HelmRelease>(raw, &at, skip, problems)),
                Release::Manifest(_) => drop(decode::<ManifestRelease>(raw, &at, skip, problems)),
                Release::Inline(_) => drop(decode::<InlineRelease>(raw, &at, skip, problems)),
                Release::Unknown => unknown(problems, &format!("{at}.type")),
            }
        }
        for (i, (input, raw)) in self.inputs.iter().zip(items(raw, "inputs")).enumerate() {
            let raw = raw.get("generate").unwrap_or(&Value::Null);
            generate(
                input.generate,
                raw,
                &format!("inputs[{i}].generate"),
                problems,
            );
        }
        for (i, addon) in self.detection.addons.iter().enumerate() {
            if addon.distribution == Distribution::Unknown {
                unknown(problems, &format!("detection.addons[{i}].distribution"));
            }
        }
        for (i, workload) in self.detection.workloads.iter().enumerate() {
            if workload.kind == WorkloadKind::Unknown {
                unknown(problems, &format!("detection.workloads[{i}].kind"));
            }
        }
    }
}

impl Strict for Template {
    fn problems(&self, raw: &Value, problems: &mut Vec<String>) {
        for (i, service) in self.services.iter().enumerate() {
            if service.pod_security == PodSecurity::Unknown {
                unknown(problems, &format!("services[{i}].podSecurity"));
            }
        }
        for (i, (input, raw)) in self.inputs.iter().zip(items(raw, "inputs")).enumerate() {
            let raw = raw.get("generate").unwrap_or(&Value::Null);
            generate(
                input.generate,
                raw,
                &format!("inputs[{i}].generate"),
                problems,
            );
        }
    }
}

impl Strict for SelectCase {
    fn problems(&self, raw: &Value, problems: &mut Vec<String>) {
        for (i, c) in self.components.iter().enumerate() {
            let at = format!("components[{i}].");
            modes(c.defaults, &at, problems);
            distributions(&c.distributions, &format!("{at}distributions"), problems);
            for (j, t) in c.tested_on.iter().enumerate() {
                if t.distribution == Distribution::Unknown {
                    unknown(problems, &format!("{at}testedOn[{j}].distribution"));
                }
            }
        }
        if self.config.distribution == Distribution::Unknown {
            unknown(problems, "config.distribution");
        }
        let want = raw.get("want").unwrap_or(&Value::Null);
        for (id, left_out) in &self.want.left_out {
            let at = format!("want.leftOut.{id}");
            let raw = want
                .get("leftOut")
                .and_then(|l| l.get(id))
                .unwrap_or(&Value::Null);
            match left_out {
                LeftOut::Distributions { distributions: d } => {
                    distributions(d, &format!("{at}.distributions"), problems);
                    keys(raw, &at, &["reason", "distributions"], problems);
                }
                LeftOut::Unsupported { .. } => keys(raw, &at, &["reason", "profile"], problems),
            }
        }
        for (i, (warning, raw)) in self
            .want
            .warnings
            .iter()
            .zip(items(want, "warnings"))
            .enumerate()
        {
            let at = format!("want.warnings[{i}]");
            match warning {
                Warning::Untested { distribution, .. } => {
                    if *distribution == Distribution::Unknown {
                        unknown(problems, &format!("{at}.distribution"));
                    }
                    keys(raw, &at, &["kind", "component", "distribution"], problems);
                }
                Warning::LeftOut { .. } => keys(raw, &at, &["kind", "component"], problems),
            }
        }
        if let (Some(error), Some(raw)) = (&self.want.error, want.get("error")) {
            let allowed: &[&str] = match error {
                crate::select::SelectError::Unknown { .. } => &["kind", "name"],
                crate::select::SelectError::NeedsDisabled { .. } => {
                    &["kind", "component", "dependency"]
                }
            };
            keys(raw, "want.error", allowed, problems);
        }
    }
}

impl Strict for MeasurementFile {
    fn problems(&self, _raw: &Value, problems: &mut Vec<String>) {
        for (i, run) in self.environments.iter().enumerate() {
            if run.distribution == MeasuredDistribution::Host(Distribution::Unknown) {
                unknown(problems, &format!("environments[{i}].distribution"));
            }
            if run.result == RunResult::Unknown {
                unknown(problems, &format!("environments[{i}].result"));
            }
        }
    }
}
