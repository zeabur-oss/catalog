//! A problem the catalog's check finds: the file, the place in it and what is wrong.
//!
//! Each check names what it finds with its own enum. [`ProblemKind`] holds any of them, and only
//! their `Display` writes text.

use std::fmt;
use std::path::{Path, PathBuf};

use crate::check::StrictProblem;
use crate::common::{LicenseUrlProblem, Profile};
use crate::order::OrderError;

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
    /// The file does not pass its schema in `schema/v1/`.
    Schema(SchemaProblem),
    /// A component, alone or with the others.
    Component(ComponentProblem),
    /// A template.
    Template(TemplateProblem),
    /// How the repository writes its files: licences, languages, READMEs, Kubernetes versions.
    Policy(PolicyProblem),
    /// A Helm value no comment explains.
    Values(ValuesProblem),
    /// The objects a component renders into.
    Render(RenderProblem),
}

impl fmt::Display for ProblemKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Strict(p) => p.fmt(f),
            Self::LicenseUrl(p) => p.fmt(f),
            Self::Schema(p) => p.fmt(f),
            Self::Component(p) => p.fmt(f),
            Self::Template(p) => p.fmt(f),
            Self::Policy(p) => p.fmt(f),
            Self::Values(p) => p.fmt(f),
            Self::Render(p) => p.fmt(f),
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

impl From<SchemaProblem> for ProblemKind {
    fn from(p: SchemaProblem) -> Self {
        Self::Schema(p)
    }
}

impl From<ComponentProblem> for ProblemKind {
    fn from(p: ComponentProblem) -> Self {
        Self::Component(p)
    }
}

impl From<TemplateProblem> for ProblemKind {
    fn from(p: TemplateProblem) -> Self {
        Self::Template(p)
    }
}

impl From<PolicyProblem> for ProblemKind {
    fn from(p: PolicyProblem) -> Self {
        Self::Policy(p)
    }
}

impl From<ValuesProblem> for ProblemKind {
    fn from(p: ValuesProblem) -> Self {
        Self::Values(p)
    }
}

impl From<RenderProblem> for ProblemKind {
    fn from(p: RenderProblem) -> Self {
        Self::Render(p)
    }
}

/// One rendering of a release: on a profile, for a Kubernetes version.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Rendering {
    /// The release.
    pub release: String,
    /// The profile.
    pub profile: Profile,
    /// The Kubernetes version.
    pub version: String,
}

impl fmt::Display for Rendering {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let profile = self.profile.as_str();
        write!(f, "{} {profile} {}", self.release, self.version)
    }
}

/// What rendering a component and checking its objects finds.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RenderProblem {
    /// The release does not render.
    Failed {
        /// The rendering.
        on: Rendering,
        /// Why.
        message: String,
    },
    /// A CRD of the objects gives no schema for kubeconform.
    CrdSchemas {
        /// The rendering.
        on: Rendering,
        /// Why.
        message: String,
    },
    /// kubeconform refuses objects rendered for a Kubernetes version, or does not run.
    Kubeconform {
        /// What kubeconform reports.
        message: String,
    },
}

impl fmt::Display for RenderProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Failed { on, message } | Self::CrdSchemas { on, message } => {
                write!(f, "{on}: {message}")
            }
            Self::Kubeconform { message } => write!(f, "kubeconform: {message}"),
        }
    }
}

/// What the check of a component's Helm values finds: every value says why it is set.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ValuesProblem {
    /// No comment explains the value at this path inside the values.
    Unexplained {
        /// The path of the value inside the values, such as `controller.replicas`.
        value: FieldPath,
    },
    /// The values are no YAML.
    NotYaml {
        /// The parser's message.
        message: String,
    },
}

impl fmt::Display for ValuesProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Unexplained { value } => write!(f, "{value} has no comment saying why"),
            Self::NotYaml { message } => write!(f, "invalid YAML: {message}"),
        }
    }
}

/// What the repository's policy finds: the files install without it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PolicyProblem {
    /// A licence is not OSI-approved.
    NotOsiApproved {
        /// The SPDX id.
        license: String,
    },
    /// A licence is opt-in only, and the component is recommended on a profile.
    OptInRecommended {
        /// The SPDX id.
        license: String,
        /// The profile.
        profile: Profile,
    },
    /// A `locales` map lacks a language that `config/locales.yaml` requires.
    MissingLocale {
        /// The language.
        lang: String,
    },
    /// The policy has no README headings for a language that `config/locales.yaml` requires.
    NoHeadings {
        /// The language.
        lang: String,
    },
    /// The README of a required language is missing.
    MissingReadme,
    /// A README has a level-1 heading.
    Level1Heading {
        /// The line.
        line: String,
        /// The first level-2 heading.
        first: String,
    },
    /// A level-2 heading is not one of the README's, or out of order.
    Heading {
        /// The line.
        line: String,
        /// The level-2 headings, in order.
        headings: Vec<String>,
    },
    /// A README lacks a section it needs.
    MissingSection {
        /// The heading.
        heading: String,
    },
    /// A template README does not say where to read a generated input.
    SecretNotNamed {
        /// The section that should name it.
        section: String,
        /// The Secret.
        secret: String,
        /// The input.
        input: String,
    },
    /// `kubeVersion` leaves out a version that `config/kubernetes.yaml` tests.
    LeavesOut {
        /// The range.
        range: String,
        /// The version.
        version: String,
    },
    /// `config/kubernetes.yaml` names something that is no version.
    NotAVersion {
        /// The text.
        version: String,
    },
    /// `kubeVersion` is no range the check reads.
    NotARange {
        /// The text.
        range: String,
    },
}

impl fmt::Display for PolicyProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::NotOsiApproved { license } => write!(f, "{license} is not OSI-approved"),
            Self::OptInRecommended { license, profile } => write!(
                f,
                "{license} is opt-in only, so {} cannot be recommended",
                profile.as_str()
            ),
            Self::MissingLocale { lang } => write!(f, "no {lang}"),
            Self::NoHeadings { lang } => write!(f, "no README headings for {lang}"),
            Self::MissingReadme => f.write_str("missing"),
            Self::Level1Heading { line, first } => {
                write!(f, "level-1 heading {line:?}; start with ## {first}")
            }
            Self::Heading { line, headings } => write!(
                f,
                "heading {line:?}; the level-2 headings are {}, in that order",
                headings.join(", ")
            ),
            Self::MissingSection { heading } => write!(f, "no ## {heading}"),
            Self::SecretNotNamed {
                section,
                secret,
                input,
            } => write!(
                f,
                "{section} does not name Secret {secret} of input {input}"
            ),
            Self::LeavesOut { range, version } => write!(
                f,
                "{range} leaves out {version}, which config/kubernetes.yaml tests"
            ),
            Self::NotAVersion { version } => write!(f, "{version:?} is not a version"),
            Self::NotARange { range } => write!(f, "{range:?} is not a range it reads"),
        }
    }
}

/// What the check finds in a template.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TemplateProblem {
    /// A client cannot render it: its manifests do not parse, or two inputs share a name or a
    /// Secret key.
    Broken {
        /// The renderer's message.
        message: String,
    },
    /// The directory name is no DNS label of at most 49 characters, so the e2e namespace
    /// `templates-e2e-<id>` is none either.
    Id,
    /// A component has the same ID.
    SameIdAsComponent,
    /// Two services have this name.
    DuplicateService {
        /// The name.
        name: String,
    },
    /// `metadata.version` has no `# renovate: depName=<image>` comment on the line above it.
    VersionWithoutImage,
    /// `metadata.version` is not the tag, pinned with a digest, of the image it names in the
    /// manifests.
    VersionNotPinned {
        /// The image.
        image: String,
    },
    /// What the template requires has no install order on a profile.
    Requires {
        /// The profile.
        profile: Profile,
        /// Why.
        error: OrderError,
    },
}

impl fmt::Display for TemplateProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Broken { message } => f.write_str(message),
            Self::Id => {
                f.write_str("the directory name must be a DNS label of at most 49 characters")
            }
            Self::SameIdAsComponent => f.write_str("a component has the same ID"),
            Self::DuplicateService { name } => write!(f, "duplicate service {name}"),
            Self::VersionWithoutImage => {
                f.write_str(r##"needs "# renovate: depName=<image>" on the line above it"##)
            }
            Self::VersionNotPinned { image } => write!(
                f,
                "must be the tag of {image} in manifests.yaml, pinned with a digest"
            ),
            Self::Requires { profile, error } => write!(f, "on {}: {error}", profile.as_str()),
        }
    }
}

/// A rule of a schema that a value breaks.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SchemaProblem {
    /// The validator's message.
    pub message: String,
}

impl fmt::Display for SchemaProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

/// What the check finds in a component, alone or with the others.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ComponentProblem {
    /// The directory name is no id.
    Id,
    /// `provides.capability` is the id of another component, which an installer reads first.
    CapabilityIsComponent {
        /// The capability.
        capability: String,
    },
    /// `urlDigests` has not one digest for each URL.
    UrlDigests {
        /// How many URLs the release has.
        urls: usize,
    },
    /// An input has a path and the component no chart release for it.
    InputWithoutRelease,
    /// A required input has no value CI can install it with.
    InputWithoutValue,
    /// `provides.registry.ca` is in a namespace the component does not own.
    CaNamespace {
        /// The namespace.
        namespace: String,
    },
    /// An internal CRD is not in `detection.crds`.
    InternalCrd {
        /// The CRD.
        crd: String,
    },
    /// An input generated as a cluster IP has no path.
    ClusterIpWithoutPath,
    /// `networkPolicies` is no list of objects.
    NetworkPolicies {
        /// The parser's message.
        message: String,
    },
    /// An object of `networkPolicies` is no `NetworkPolicy` in a namespace the component owns.
    NetworkPolicy {
        /// Its kind.
        kind: String,
        /// Its name.
        name: String,
    },
    /// Two works have the same name.
    WorkTwice {
        /// The name.
        name: String,
    },
    /// A quantity of a work.
    Quantity(QuantityProblem),
    /// The component contributes values and has no Helm release, by which an installer finds it.
    ContributorWithoutHelm,
    /// A contribution names no component.
    UnknownTarget {
        /// The id it names.
        component: String,
    },
    /// A contribution names a release its component does not have.
    UnknownTargetRelease {
        /// The component.
        component: String,
        /// The release it names.
        release: String,
    },
    /// The components have no install order on a profile.
    InstallOrder {
        /// The profile.
        profile: Profile,
        /// Why.
        error: OrderError,
    },
}

impl fmt::Display for ComponentProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Id => f.write_str(
                "the directory name must be lowercase letters, digits and single hyphens",
            ),
            Self::CapabilityIsComponent { capability } => {
                write!(f, "{capability} is the ID of another component")
            }
            Self::UrlDigests { urls } => {
                write!(f, "needs one digest for each of the {urls} urls")
            }
            Self::InputWithoutRelease => f.write_str("no chart release for its path"),
            Self::InputWithoutValue => {
                f.write_str("required, so set default, testValue or generate for CI to install it")
            }
            Self::CaNamespace { namespace } => write!(f, "{namespace} is none of its namespaces"),
            Self::InternalCrd { crd } => write!(f, "{crd} is not in detection.crds"),
            Self::ClusterIpWithoutPath => f.write_str("generate clusterIP needs a path"),
            Self::NetworkPolicies { message } => f.write_str(message),
            Self::NetworkPolicy { kind, name } => write!(
                f,
                "{kind} {name} must be a NetworkPolicy in one of its namespaces"
            ),
            Self::WorkTwice { name } => write!(f, "{name} is listed twice"),
            Self::Quantity(p) => p.fmt(f),
            Self::ContributorWithoutHelm => f.write_str("needs a helm release of its own"),
            Self::UnknownTarget { component } => write!(f, "unknown target {component}"),
            Self::UnknownTargetRelease { component, release } => {
                write!(f, "{component} has no release {release}")
            }
            Self::InstallOrder { profile, error } => write!(f, "{}: {error}", profile.as_str()),
        }
    }
}

/// A quantity that is not one, or not above zero.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum QuantityProblem {
    /// It is no Kubernetes quantity.
    NotAQuantity {
        /// The text.
        value: String,
    },
    /// It is zero or less.
    NotAboveZero {
        /// The text.
        value: String,
    },
}

impl fmt::Display for QuantityProblem {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::NotAQuantity { value } => write!(f, "{value} is not a quantity"),
            Self::NotAboveZero { value } => write!(f, "{value} is not above zero"),
        }
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
