//! Strict reading for the catalog's own check.
//!
//! Readers of the API take the types as they are: they ignore a field they do not know and read
//! an unknown value as `Unknown` [releases]. The files authors write are held to more: here a
//! field or value the types do not know is an error, named by its path, such as
//! `health[0].conditon`.
//!
//! [releases]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/releases.mdx
#![expect(
    clippy::disallowed_types,
    clippy::disallowed_methods,
    reason = "the check walks the file as written, of any shape, to find what the types skip"
)]

use std::fmt;
use std::path::Path;

use serde::de::DeserializeOwned;
use serde_json::Value;

use crate::api::{MeasuredDistribution, MeasurementFile, RunResult};
use crate::common::{Distribution, Generate, Mode, PerProfile, PodSecurity, WorkloadKind};
use crate::component::{Component, HelmRelease, InlineRelease, ManifestRelease, Release};
use crate::problem::{FieldPath, Problem, Problems};
use crate::select::{LeftOut, SelectCase, SelectError, Warning};
use crate::template::Template;
use crate::{ReadError, read_text};

/// What strict reading finds.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StrictProblem {
    /// A field the type does not have.
    UnknownField,
    /// A value the type reads as `Unknown`.
    UnknownValue,
    /// The text is no valid file of the type.
    Decode {
        /// The parser's message.
        message: String,
    },
}

impl fmt::Display for StrictProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::UnknownField => f.write_str("unknown field"),
            Self::UnknownValue => f.write_str("unknown value"),
            Self::Decode { message } => f.write_str(message),
        }
    }
}

/// A file type the check reads strictly.
pub trait Strict: DeserializeOwned {
    /// Adds each value the types read as `Unknown`, and each unknown field inside an internally
    /// tagged part of `raw`, which `serde_ignored` cannot see.
    fn problems(&self, raw: &Value, problems: &mut Problems);
}

/// Reads `text`, the content of `file`, as `T`, failing on every field and value `T` does not
/// know.
///
/// # Errors
///
/// The text is no valid `T`, or it has fields or values `T` does not know.
pub fn from_yaml<T: Strict>(file: &Path, text: &str) -> Result<T, Vec<Problem>> {
    let mut problems = Problems::new(file);
    let raw: Value = match serde_saphyr::from_str(text) {
        Ok(raw) => raw,
        Err(e) => {
            let message = e.to_string();
            problems.push(FieldPath::root(), StrictProblem::Decode { message });
            return Err(problems.into_vec());
        }
    };
    let Some(read) = decode::<T>(&raw, &FieldPath::root(), &[], &mut problems) else {
        return Err(problems.into_vec());
    };
    read.problems(&raw, &mut problems);
    if problems.is_empty() {
        Ok(read)
    } else {
        Err(problems.into_vec())
    }
}

/// Reads the file at `path` strictly.
///
/// # Errors
///
/// It cannot be read, is no valid `T`, or has fields or values `T` does not know.
pub fn read<T: Strict>(path: &Path) -> Result<T, ReadError> {
    from_yaml(path, &read_text(path)?).map_err(ReadError::Check)
}

/// Decodes `raw`, adding each ignored field below `at` except the top-level fields `skip` to
/// `problems`. `None` with the error in `problems` when it does not decode.
fn decode<T: DeserializeOwned>(
    raw: &Value,
    at: &FieldPath,
    skip: &[&str],
    problems: &mut Problems,
) -> Option<T> {
    let mut ignored = Vec::new();
    let read = serde_ignored::deserialize(raw.clone(), |path| ignored.push(field_path(&path)));
    for path in ignored {
        if !skip.iter().any(|key| path == FieldPath::root().key(*key)) {
            problems.push(at.clone().join(path), StrictProblem::UnknownField);
        }
    }
    match read {
        Ok(read) => Some(read),
        Err(e) => {
            let message = e.to_string();
            problems.push(at.clone(), StrictProblem::Decode { message });
            None
        }
    }
}

fn field_path(path: &serde_ignored::Path<'_>) -> FieldPath {
    match path {
        serde_ignored::Path::Root => FieldPath::root(),
        serde_ignored::Path::Seq { parent, index } => field_path(parent).index(*index),
        serde_ignored::Path::Map { parent, key } => field_path(parent).key(key.as_str()),
        serde_ignored::Path::Some { parent }
        | serde_ignored::Path::NewtypeStruct { parent }
        | serde_ignored::Path::NewtypeVariant { parent } => field_path(parent),
    }
}

fn items<'a>(raw: &'a Value, key: &str) -> impl Iterator<Item = &'a Value> {
    raw.get(key).and_then(Value::as_array).into_iter().flatten()
}

/// Checks the keys of a tagged part that has no struct of its own.
fn keys(raw: &Value, at: &FieldPath, allowed: &[&str], problems: &mut Problems) {
    for key in raw.as_object().into_iter().flat_map(|m| m.keys()) {
        if !allowed.contains(&key.as_str()) {
            problems.push(at.clone().key(key.as_str()), StrictProblem::UnknownField);
        }
    }
}

fn modes(defaults: PerProfile<Mode>, at: &FieldPath, problems: &mut Problems) {
    for (profile, mode) in [
        ("single-node", defaults.single_node),
        ("multi-node", defaults.multi_node),
    ] {
        if mode == Mode::Unknown {
            let at = at.clone().key("defaults").key(profile);
            problems.push(at, StrictProblem::UnknownValue);
        }
    }
}

fn distributions(list: &[Distribution], at: &FieldPath, problems: &mut Problems) {
    for (i, d) in list.iter().enumerate() {
        if *d == Distribution::Unknown {
            problems.push(at.clone().index(i), StrictProblem::UnknownValue);
        }
    }
}

fn generate(generate: Option<Generate>, raw: &Value, at: &FieldPath, problems: &mut Problems) {
    match generate {
        Some(Generate::Unknown) => unknown(problems, at.clone().key("type")),
        Some(Generate::Random { .. }) => keys(raw, at, &["type", "bytes", "encoding"], problems),
        Some(Generate::ClusterIp) => keys(raw, at, &["type"], problems),
        None => {}
    }
}

fn inputs(generates: impl Iterator<Item = Option<Generate>>, raw: &Value, problems: &mut Problems) {
    for (i, (input, raw)) in generates.zip(items(raw, "inputs")).enumerate() {
        let raw = raw.get("generate").unwrap_or(&Value::Null);
        let at = FieldPath::root().key("inputs").index(i).key("generate");
        generate(input, raw, &at, problems);
    }
}

fn unknown(problems: &mut Problems, at: FieldPath) {
    problems.push(at, StrictProblem::UnknownValue);
}

impl Strict for Component {
    fn problems(&self, raw: &Value, problems: &mut Problems) {
        let root = FieldPath::root();
        modes(self.defaults, &root, problems);
        distributions(
            &self.distributions,
            &root.clone().key("distributions"),
            problems,
        );
        if self.security.pod_security == PodSecurity::Unknown {
            unknown(problems, root.clone().key("security").key("podSecurity"));
        }
        for (i, (release, raw)) in self.releases.iter().zip(items(raw, "releases")).enumerate() {
            let at = root.clone().key("releases").index(i);
            let skip = &["type"];
            match release {
                Release::Helm(_) => drop(decode::<HelmRelease>(raw, &at, skip, problems)),
                Release::Manifest(_) => drop(decode::<ManifestRelease>(raw, &at, skip, problems)),
                Release::Inline(_) => drop(decode::<InlineRelease>(raw, &at, skip, problems)),
                Release::Unknown => unknown(problems, at.key("type")),
            }
        }
        inputs(self.inputs.iter().map(|i| i.generate), raw, problems);
        for (i, addon) in self.detection.addons.iter().enumerate() {
            if addon.distribution == Distribution::Unknown {
                let at = root.clone().key("detection").key("addons").index(i);
                unknown(problems, at.key("distribution"));
            }
        }
        for (i, workload) in self.detection.workloads.iter().enumerate() {
            if workload.kind == WorkloadKind::Unknown {
                let at = root.clone().key("detection").key("workloads").index(i);
                unknown(problems, at.key("kind"));
            }
        }
    }
}

impl Strict for Template {
    fn problems(&self, raw: &Value, problems: &mut Problems) {
        for (i, service) in self.services.iter().enumerate() {
            if service.pod_security == PodSecurity::Unknown {
                let at = FieldPath::root().key("services").index(i);
                unknown(problems, at.key("podSecurity"));
            }
        }
        inputs(self.inputs.iter().map(|i| i.generate), raw, problems);
    }
}

impl Strict for SelectCase {
    fn problems(&self, raw: &Value, problems: &mut Problems) {
        for (i, c) in self.components.iter().enumerate() {
            let at = FieldPath::root().key("components").index(i);
            modes(c.defaults, &at, problems);
            distributions(&c.distributions, &at.clone().key("distributions"), problems);
            for (j, t) in c.tested_on.iter().enumerate() {
                if t.distribution == Distribution::Unknown {
                    let at = at.clone().key("testedOn").index(j);
                    unknown(problems, at.key("distribution"));
                }
            }
        }
        if self.config.distribution == Distribution::Unknown {
            unknown(
                problems,
                FieldPath::root().key("config").key("distribution"),
            );
        }
        let want = raw.get("want").unwrap_or(&Value::Null);
        let want_at = FieldPath::root().key("want");
        for (id, left_out) in &self.want.left_out {
            let at = want_at.clone().key("leftOut").key(id.as_str());
            let raw = want
                .get("leftOut")
                .and_then(|l| l.get(id))
                .unwrap_or(&Value::Null);
            match left_out {
                LeftOut::Distributions { distributions: d } => {
                    distributions(d, &at.clone().key("distributions"), problems);
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
            let at = want_at.clone().key("warnings").index(i);
            match warning {
                Warning::Untested { distribution, .. } => {
                    if *distribution == Distribution::Unknown {
                        unknown(problems, at.clone().key("distribution"));
                    }
                    keys(raw, &at, &["kind", "component", "distribution"], problems);
                }
                Warning::LeftOut { .. } => keys(raw, &at, &["kind", "component"], problems),
            }
        }
        if let (Some(error), Some(raw)) = (&self.want.error, want.get("error")) {
            let allowed: &[&str] = match error {
                SelectError::Unknown { .. } => &["kind", "name"],
                SelectError::NeedsDisabled { .. } => &["kind", "component", "dependency"],
            };
            keys(raw, &want_at.key("error"), allowed, problems);
        }
    }
}

impl Strict for MeasurementFile {
    fn problems(&self, _raw: &Value, problems: &mut Problems) {
        for (i, run) in self.environments.iter().enumerate() {
            let at = FieldPath::root().key("environments").index(i);
            if run.distribution == MeasuredDistribution::Host(Distribution::Unknown) {
                unknown(problems, at.clone().key("distribution"));
            }
            if run.result == RunResult::Unknown {
                unknown(problems, at.key("result"));
            }
        }
    }
}
