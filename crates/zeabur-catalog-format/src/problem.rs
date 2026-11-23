//! A problem the catalog's check finds: the file, the place in it and what is wrong.
//!
//! Each check names what it finds with its own enum. [`ProblemKind`] holds any of them, and only
//! their `Display` writes text.

use std::fmt;
use std::path::{Path, PathBuf};

use crate::check::StrictProblem;
use crate::common::LicenseUrlProblem;

/// One problem of one file.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Problem {
    /// The file, relative to the checkout, or `components` for the components together.
    pub file: PathBuf,
    /// Where in the file. Empty for the file as a whole.
    pub at: FieldPath,
    /// What is wrong.
    pub kind: ProblemKind,
}

impl fmt::Display for Problem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.at.is_empty() {
            write!(f, "{}: {}", self.file.display(), self.kind)
        } else {
            write!(f, "{}: {}: {}", self.file.display(), self.at, self.kind)
        }
    }
}

/// What a check finds.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ProblemKind {
    /// A field or value the types do not know, or text that is no valid file.
    Strict(StrictProblem),
    /// A `LicenseRef-` id and its link.
    LicenseUrl(LicenseUrlProblem),
}

impl fmt::Display for ProblemKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Strict(p) => p.fmt(f),
            Self::LicenseUrl(p) => p.fmt(f),
        }
    }
}

impl From<StrictProblem> for ProblemKind {
    fn from(p: StrictProblem) -> Self {
        Self::Strict(p)
    }
}

impl From<LicenseUrlProblem> for ProblemKind {
    fn from(p: LicenseUrlProblem) -> Self {
        Self::LicenseUrl(p)
    }
}

/// One step from a value into what it holds.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Segment {
    /// A field of an object.
    Key(String),
    /// An item of a list.
    Index(usize),
}

/// The way from the top of a file to a value, written as `health[0].condition`.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct FieldPath(Vec<Segment>);

impl FieldPath {
    /// The top of the file.
    #[must_use]
    pub const fn root() -> Self {
        Self(Vec::new())
    }

    /// The field `key` of the value here.
    #[must_use]
    pub fn key(mut self, key: impl Into<String>) -> Self {
        self.0.push(Segment::Key(key.into()));
        self
    }

    /// The item `index` of the list here.
    #[must_use]
    pub fn index(mut self, index: usize) -> Self {
        self.0.push(Segment::Index(index));
        self
    }

    /// This path, then `rest`.
    #[must_use]
    pub fn join(mut self, rest: Self) -> Self {
        self.0.extend(rest.0);
        self
    }

    /// Whether this is the top of the file.
    #[must_use]
    pub const fn is_empty(&self) -> bool {
        self.0.is_empty()
    }
}

impl fmt::Display for FieldPath {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for (i, segment) in self.0.iter().enumerate() {
            match segment {
                Segment::Key(key) if i == 0 => write!(f, "{key}")?,
                Segment::Key(key) => write!(f, ".{key}")?,
                Segment::Index(index) => write!(f, "[{index}]")?,
            }
        }
        Ok(())
    }
}

/// The problems of one file, as a check finds them.
#[derive(Debug)]
pub struct Problems {
    file: PathBuf,
    found: Vec<Problem>,
}

impl Problems {
    /// No problems yet in `file`.
    #[must_use]
    pub fn new(file: &Path) -> Self {
        Self {
            file: file.to_owned(),
            found: Vec::new(),
        }
    }

    /// Adds `kind` at `at`.
    pub fn push(&mut self, at: FieldPath, kind: impl Into<ProblemKind>) {
        self.found.push(Problem {
            file: self.file.clone(),
            at,
            kind: kind.into(),
        });
    }

    /// Whether none were found.
    #[must_use]
    pub const fn is_empty(&self) -> bool {
        self.found.is_empty()
    }

    /// The problems found, in order.
    #[must_use]
    pub fn into_vec(self) -> Vec<Problem> {
        self.found
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::{FieldPath, Problem};
    use crate::check::StrictProblem;

    #[test]
    fn a_path_reads_as_its_keys_and_indexes() {
        let at = FieldPath::root().key("health").index(0).key("condition");
        assert_eq!(at.to_string(), "health[0].condition");
        assert_eq!(FieldPath::root().index(1).key("a").to_string(), "[1].a");
    }

    #[test]
    fn a_problem_names_its_file_and_place() {
        let problem = Problem {
            file: PathBuf::from("components/web/component.yaml"),
            at: FieldPath::root().key("distributions").index(1),
            kind: StrictProblem::UnknownValue.into(),
        };
        assert_eq!(
            problem.to_string(),
            "components/web/component.yaml: distributions[1]: unknown value"
        );
    }
}
