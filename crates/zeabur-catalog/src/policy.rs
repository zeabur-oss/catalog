//! How the catalog's files are written: licences and their links, languages, READMEs, a comment
//! on every Helm value and the Kubernetes versions a component admits. The files install without
//! these; the repository holds itself to them.
#![expect(
    clippy::disallowed_types,
    reason = "the languages of every `locales` map are found in the file as written, of any shape"
)]

use std::collections::BTreeMap;
use std::fs;
use std::path::Path;

use serde::Deserialize;
use serde_json::Value;
use zeabur_catalog_engine::kube_version;
use zeabur_catalog_format::common::{
    LicenseUrlProblem, LicenseUrls, Mode, Profile, license_url_problems,
};
use zeabur_catalog_format::problem::{FieldPath, PolicyProblem, Problem, Problems, ValuesProblem};
use zeabur_catalog_format::{Component, Template};

use crate::values;

/// What `config/` sets for the policy.
#[derive(Debug)]
pub(crate) struct Config {
    licenses: Licenses,
    locales: Vec<String>,
    kube_versions: Vec<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Licenses {
    osi_approved: Vec<String>,
    #[serde(default)]
    opt_in_only: Vec<String>,
}

#[derive(Deserialize)]
struct Locales {
    required: Vec<String>,
}

/// `config/kubernetes.yaml`: the versions that rendering, the e2e and each distribution test.
#[derive(Deserialize)]
struct Kubernetes {
    versions: Vec<String>,
    e2e: Pinned,
    k3s: Pinned,
    rke2: Pinned,
}

#[derive(Deserialize)]
struct Pinned {
    version: String,
}

impl Config {
    /// Reads `config/` under `root`.
    ///
    /// # Errors
    ///
    /// A file cannot be read or parsed.
    pub(crate) fn read(root: &Path) -> Result<Self, String> {
        let licenses: Licenses = read_yaml(root, "config/licenses.yaml")?;
        let locales: Locales = read_yaml(root, "config/locales.yaml")?;
        let kubernetes: Kubernetes = read_yaml(root, "config/kubernetes.yaml")?;
        let mut kube_versions = kubernetes.versions;
        kube_versions.extend([
            kubernetes.e2e.version,
            kubernetes.k3s.version,
            kubernetes.rke2.version,
        ]);
        Ok(Self {
            licenses,
            locales: locales.required,
            kube_versions,
        })
    }
}

fn read_yaml<T: for<'de> Deserialize<'de>>(root: &Path, path: &str) -> Result<T, String> {
    let text = fs::read_to_string(root.join(path)).map_err(|e| format!("{path}: {e}"))?;
    serde_saphyr::from_str(&text).map_err(|e| format!("{path}: {e}"))
}

/// The level-2 headings of a component README by language (`""` is English), in order. The
/// second and fifth are optional; the second is required when an input needs a value from outside.
const COMPONENT_SECTIONS: &[(&str, [&str; 5])] = &[
    (
        "",
        [
            "Introduction",
            "Before you install",
            "Use it",
            "Remove it",
            "Troubleshooting",
        ],
    ),
    (
        "zh-TW",
        ["簡介", "安裝前準備", "使用方式", "移除", "疑難排解"],
    ),
    (
        "zh-CN",
        ["简介", "安装前准备", "使用方法", "移除", "故障排除"],
    ),
];

/// The level-2 headings of a template README by language, in order. Every README has the first
/// two.
const TEMPLATE_SECTIONS: &[(&str, [&str; 4])] = &[
    (
        "",
        [
            "Introduction",
            "Getting started",
            "Configuration",
            "Troubleshooting",
        ],
    ),
    ("zh-TW", ["簡介", "開始使用", "設定", "疑難排解"]),
    ("zh-CN", ["简介", "开始使用", "配置", "故障排除"]),
];

/// The policy problems of the component in `dir` under `root`, read as `component` and as
/// `raw`.
pub(crate) fn component_problems(
    config: &Config,
    root: &Path,
    dir: &Path,
    component: &Component,
    raw: &Value,
) -> Vec<Problem> {
    let mut found = Problems::new(&dir.join("component.yaml"));
    license_problems(&config.licenses, component, &mut found);
    let metadata = &component.metadata;
    license_urls(&metadata.license, &metadata.license_urls, &mut found);
    missing_locales(raw, &config.locales, &mut found);
    let mut out = kube_version_problems(config, &component.kube_version, &mut found);
    let needs_setup = component
        .inputs
        .iter()
        .any(|i| i.required && i.default.is_none() && i.generate.is_none());
    for (lang, want, text) in readmes(root, dir, &config.locales, COMPONENT_SECTIONS, &mut out) {
        let mut required = vec![want[0], want[2], want[3]];
        if needs_setup {
            required.push(want[1]);
        }
        let mut readme_found = Problems::new(&dir.join(readme(lang)));
        readme_sections(&text, &want, &required, &mut readme_found);
        out.extend(readme_found.into_vec());
    }
    helm_values_problems(component, &mut found);
    let mut problems = found.into_vec();
    problems.extend(out);
    problems
}

/// The problem of `range` against the versions `config/kubernetes.yaml` tests: the first version
/// it leaves out or cannot read.
fn kube_version_problems(config: &Config, range: &str, found: &mut Problems) -> Vec<Problem> {
    let at = FieldPath::root().key("kubeVersion");
    for version in &config.kube_versions {
        match kube_version::admits(range, version) {
            Some(true) => {}
            Some(false) => {
                let (range, version) = (range.to_owned(), version.clone());
                found.push(at, PolicyProblem::LeavesOut { range, version });
                break;
            }
            None if kube_version::release(version).is_none() => {
                let mut config_found = Problems::new(Path::new("config/kubernetes.yaml"));
                let version = version.clone();
                config_found.push(FieldPath::root(), PolicyProblem::NotAVersion { version });
                return config_found.into_vec();
            }
            None => {
                let range = range.to_owned();
                found.push(at, PolicyProblem::NotARange { range });
                break;
            }
        }
    }
    Vec::new()
}

/// Adds a problem for each Helm value of `component` that no comment explains.
fn helm_values_problems(component: &Component, found: &mut Problems) {
    for (i, release) in component.releases.iter().enumerate() {
        let Some(helm) = release.helm() else {
            continue;
        };
        let at = FieldPath::root().key("releases").index(i);
        if let Some(values) = &helm.values {
            unexplained(values, &at.clone().key("values"), found);
        }
        for profile in Profile::ALL {
            if let Some(values) = helm.profiles.get(profile) {
                let at = at.clone().key("profiles").key(profile.as_str());
                unexplained(values, &at, found);
            }
        }
    }
    for (i, contribution) in component.contributes.iter().enumerate() {
        let at = FieldPath::root().key("contributes").index(i).key("values");
        unexplained(&contribution.values, &at, found);
    }
}

/// Adds a problem for each value in `values`, the field at `at`, that no comment explains.
fn unexplained(values: &str, at: &FieldPath, found: &mut Problems) {
    match values::unexplained(values) {
        Ok(paths) => {
            for value in paths {
                found.push(at.clone(), ValuesProblem::Unexplained { value });
            }
        }
        Err(message) => found.push(at.clone(), ValuesProblem::NotYaml { message }),
    }
}

/// The policy problems of the template in `dir` under `root`.
pub(crate) fn template_problems(
    config: &Config,
    root: &Path,
    dir: &Path,
    template: &Template,
    raw: &Value,
) -> Vec<Problem> {
    let mut found = Problems::new(&dir.join("template.yaml"));
    let metadata = &template.metadata;
    license_urls(&metadata.license, &metadata.license_urls, &mut found);
    missing_locales(raw, &config.locales, &mut found);
    let mut out = found.into_vec();
    for (lang, want, text) in readmes(root, dir, &config.locales, TEMPLATE_SECTIONS, &mut out) {
        let mut readme_found = Problems::new(&dir.join(readme(lang)));
        let sections = readme_sections(&text, &want, &want[..2], &mut readme_found);
        // A generated value is only in its Secret, so the README says where to read it.
        let getting_started = sections.get(want[1]).map_or("", String::as_str);
        for input in &template.inputs {
            if let Some(secret) = &input.secret
                && input.generate.is_some()
                && !getting_started.contains(&secret.name)
            {
                let problem = PolicyProblem::SecretNotNamed {
                    section: want[1].to_owned(),
                    secret: secret.name.clone(),
                    input: input.name.clone(),
                };
                readme_found.push(FieldPath::root(), problem);
            }
        }
        out.extend(readme_found.into_vec());
    }
    out
}

/// The file name of the README in `lang`.
fn readme(lang: &str) -> String {
    if lang.is_empty() {
        "README.md".to_owned()
    } else {
        format!("README.{lang}.md")
    }
}

/// The README of each language in `dir` with its headings, adding to `out` the ones that are
/// missing or have no headings.
fn readmes<'a, const N: usize>(
    root: &Path,
    dir: &Path,
    locales: &'a [String],
    sections: &[(&str, [&'static str; N])],
    out: &mut Vec<Problem>,
) -> Vec<(&'a str, [&'static str; N], String)> {
    let mut read = Vec::new();
    for lang in std::iter::once("").chain(locales.iter().map(String::as_str)) {
        let path = dir.join(readme(lang));
        let mut found = Problems::new(&path);
        match sections.iter().find(|(l, _)| *l == lang) {
            None => {
                let lang = lang.to_owned();
                found.push(FieldPath::root(), PolicyProblem::NoHeadings { lang });
            }
            Some((_, want)) => match fs::read_to_string(root.join(&path)) {
                Ok(text) => read.push((lang, *want, text)),
                Err(_) => found.push(FieldPath::root(), PolicyProblem::MissingReadme),
            },
        }
        out.extend(found.into_vec());
    }
    read
}

/// The text of each level-2 section of a Markdown file. Adds a problem for each level-1 heading,
/// each level-2 heading outside `want` or out of its order, and each heading of `required` that
/// is missing.
fn readme_sections(
    text: &str,
    want: &[&str],
    required: &[&str],
    found: &mut Problems,
) -> BTreeMap<String, String> {
    let mut sections = BTreeMap::new();
    let mut current: Option<String> = None;
    let mut last = None;
    let mut fenced = false;
    for line in text.split('\n') {
        if line.starts_with("```") {
            fenced = !fenced;
        }
        if !fenced {
            if line.starts_with("# ") {
                let problem = PolicyProblem::Level1Heading {
                    line: line.to_owned(),
                    first: want.first().map_or_else(String::new, |w| (*w).to_owned()),
                };
                found.push(FieldPath::root(), problem);
                continue;
            }
            if let Some(heading) = line.strip_prefix("## ") {
                let heading = heading.trim();
                let index = want.iter().position(|w| *w == heading);
                if index.is_none() || index <= last {
                    let problem = PolicyProblem::Heading {
                        line: line.to_owned(),
                        headings: want.iter().map(|w| (*w).to_owned()).collect(),
                    };
                    found.push(FieldPath::root(), problem);
                }
                last = last.max(index);
                sections.insert(heading.to_owned(), String::new());
                current = Some(heading.to_owned());
                continue;
            }
        }
        if let Some(section) = current.as_ref().and_then(|c| sections.get_mut(c)) {
            section.push_str(line);
            section.push('\n');
        }
    }
    for heading in required {
        if !sections.contains_key(*heading) {
            let heading = (*heading).to_owned();
            found.push(FieldPath::root(), PolicyProblem::MissingSection { heading });
        }
    }
    sections
}

/// A problem for each licence of the component that is not OSI-approved, or that is opt-in only
/// on a profile that recommends the component.
fn license_problems(licenses: &Licenses, component: &Component, found: &mut Problems) {
    let at = FieldPath::root().key("metadata").key("license");
    for license in not_osi_approved(licenses, &component.metadata.license) {
        if !licenses.opt_in_only.contains(&license) {
            found.push(at.clone(), PolicyProblem::NotOsiApproved { license });
            continue;
        }
        for profile in Profile::ALL {
            if component.mode(profile) == Mode::Recommended {
                let license = license.clone();
                found.push(
                    at.clone(),
                    PolicyProblem::OptInRecommended { license, profile },
                );
            }
        }
    }
}

/// Adds a problem for each `LicenseRef-` id of `license` without a link, and each link to an id
/// it does not use.
fn license_urls(license: &str, urls: &LicenseUrls, found: &mut Problems) {
    let at = FieldPath::root().key("metadata");
    for problem in license_url_problems(license, urls) {
        let at = match &problem {
            LicenseUrlProblem::Unlinked { .. } => at.clone().key("license"),
            LicenseUrlProblem::Unused { id } => at.clone().key("licenseUrls").key(id.as_str()),
        };
        found.push(at, problem);
    }
}

/// The identifiers of an SPDX expression that are not OSI-approved. Every identifier counts,
/// both sides of an `OR` too.
fn not_osi_approved(licenses: &Licenses, expression: &str) -> Vec<String> {
    let mut out: Vec<String> = Vec::new();
    for token in expression.split(|c: char| c.is_whitespace() || c == '(' || c == ')') {
        let id = token.strip_suffix('+').unwrap_or(token);
        if !id.is_empty()
            && !["AND", "OR", "WITH"].contains(&id)
            && !licenses.osi_approved.iter().any(|a| a == id)
            && !out.iter().any(|o| o == id)
        {
            out.push(id.to_owned());
        }
    }
    out
}

/// Adds a problem for each `locales` map in `doc` that lacks a required language.
fn missing_locales(doc: &Value, required: &[String], found: &mut Problems) {
    walk_locales(doc, &FieldPath::root(), required, found);
}

fn walk_locales(value: &Value, at: &FieldPath, required: &[String], found: &mut Problems) {
    match value {
        Value::Object(map) => {
            if let Some(Value::Object(locales)) = map.get("locales") {
                for lang in required {
                    if !locales.contains_key(lang) {
                        let lang = lang.clone();
                        found.push(
                            at.clone().key("locales"),
                            PolicyProblem::MissingLocale { lang },
                        );
                    }
                }
            }
            for (key, child) in map {
                walk_locales(child, &at.clone().key(key.as_str()), required, found);
            }
        }
        Value::Array(items) => {
            for (i, child) in items.iter().enumerate() {
                walk_locales(child, &at.clone().index(i), required, found);
            }
        }
        Value::Null | Value::Bool(_) | Value::Number(_) | Value::String(_) => {}
    }
}

#[cfg(test)]
mod tests {
    use zeabur_catalog_format::component::Release;
    use zeabur_catalog_format::problem::ProblemKind;

    use super::*;

    fn licenses() -> Licenses {
        Licenses {
            osi_approved: vec!["MIT".to_owned(), "Apache-2.0".to_owned()],
            opt_in_only: vec!["BUSL-1.1".to_owned()],
        }
    }

    #[test]
    fn every_licence_of_an_expression_must_be_approved() {
        let l = licenses();
        assert_eq!(
            not_osi_approved(&l, "MIT OR Apache-2.0"),
            Vec::<String>::new()
        );
        assert_eq!(not_osi_approved(&l, "(MIT OR SSPL-1.0)"), ["SSPL-1.0"]);
        assert_eq!(
            not_osi_approved(&l, "Apache-2.0+ AND BUSL-1.1"),
            ["BUSL-1.1"]
        );
    }

    #[test]
    fn an_opt_in_licence_cannot_be_recommended() {
        let root = Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../.."));
        let catalog = zeabur_catalog_format::Catalog::read(root).unwrap();
        let mut component = catalog.components["traefik"].clone();
        component.metadata.license = "BUSL-1.1".to_owned();
        let kinds = |component: &Component| {
            let mut found = Problems::new(Path::new("component.yaml"));
            license_problems(&licenses(), component, &mut found);
            found
                .into_vec()
                .into_iter()
                .map(|p| p.kind)
                .collect::<Vec<_>>()
        };
        let want = PolicyProblem::OptInRecommended {
            license: "BUSL-1.1".to_owned(),
            profile: Profile::SingleNode,
        };
        let got = kinds(&component);
        assert!(got.contains(&want.into()), "{got:?}");
        component.metadata.license = "SSPL-1.0".to_owned();
        let want = PolicyProblem::NotOsiApproved {
            license: "SSPL-1.0".to_owned(),
        };
        assert_eq!(kinds(&component), [want.into()]);
    }

    #[test]
    fn every_helm_value_says_why() {
        let root = Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../.."));
        let catalog = zeabur_catalog_format::Catalog::read(root).unwrap();
        let mut component = catalog.components["traefik"].clone();
        let (i, helm) = component
            .releases
            .iter_mut()
            .enumerate()
            .find_map(|(i, r)| match r {
                Release::Helm(helm) => Some((i, helm)),
                Release::Manifest(_) | Release::Inline(_) | Release::Unknown => None,
            })
            .unwrap();
        helm.values = Some("# why\na: 1\nb:\n  c: 2\n".to_owned());
        helm.profiles.multi_node = Some("d: [1\n".to_owned());
        let mut found = Problems::new(Path::new("component.yaml"));
        helm_values_problems(&component, &mut found);
        let got: Vec<_> = found
            .into_vec()
            .into_iter()
            .map(|p| (p.at, p.kind))
            .collect();
        let release = FieldPath::root().key("releases").index(i);
        let value = FieldPath::root().key("b").key("c");
        assert_eq!(
            got[0],
            (
                release.clone().key("values"),
                ValuesProblem::Unexplained { value }.into()
            )
        );
        assert_eq!(got.len(), 2, "{got:?}");
        assert!(matches!(
            &got[1],
            (at, ProblemKind::Values(ValuesProblem::NotYaml { .. }))
                if *at == release.key("profiles").key("multi-node")
        ));
    }

    #[test]
    fn every_locales_map_has_the_required_languages() {
        let doc = serde_json::json!({
            "metadata": {"locales": {"zh-TW": {}}},
            "inputs": [{"locales": {"zh-TW": {}, "zh-CN": {}}}, {"locales": {}}],
        });
        let required = ["zh-TW".to_owned(), "zh-CN".to_owned()];
        let mut found = Problems::new(Path::new("component.yaml"));
        missing_locales(&doc, &required, &mut found);
        let got: Vec<(String, ProblemKind)> = found
            .into_vec()
            .into_iter()
            .map(|p| (p.at.to_string(), p.kind))
            .collect();
        let missing = |at: &str, lang: &str| {
            let lang = lang.to_owned();
            (at.to_owned(), PolicyProblem::MissingLocale { lang }.into())
        };
        assert_eq!(
            got,
            [
                missing("inputs[1].locales", "zh-TW"),
                missing("inputs[1].locales", "zh-CN"),
                missing("metadata.locales", "zh-CN"),
            ]
        );
    }

    #[test]
    fn readme_headings_come_in_order() {
        let want = ["A", "B", "C"];
        let text = "intro\n## A\na\n```\n# not a heading\n```\n## C\nc\n";
        let kinds = |found: Problems| -> Vec<ProblemKind> {
            found.into_vec().into_iter().map(|p| p.kind).collect()
        };
        let mut found = Problems::new(Path::new("README.md"));
        let sections = readme_sections(text, &want, &["A", "B"], &mut found);
        let heading = "B".to_owned();
        assert_eq!(
            kinds(found),
            [PolicyProblem::MissingSection { heading }.into()]
        );
        assert_eq!(sections["A"], "a\n```\n# not a heading\n```\n");
        let mut found = Problems::new(Path::new("README.md"));
        readme_sections("# Title\n## C\n## A\n## D\n", &want, &[], &mut found);
        let headings: Vec<String> = want.iter().map(|w| (*w).to_owned()).collect();
        let heading = |line: &str| {
            let line = line.to_owned();
            let headings = headings.clone();
            PolicyProblem::Heading { line, headings }.into()
        };
        assert_eq!(
            kinds(found),
            [
                PolicyProblem::Level1Heading {
                    line: "# Title".to_owned(),
                    first: "A".to_owned()
                }
                .into(),
                heading("## A"),
                heading("## D"),
            ]
        );
    }

    #[test]
    fn a_template_readme_has_the_sections_and_names_generated_secrets() {
        let root = Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../.."));
        let config = Config::read(root).unwrap();
        let catalog = zeabur_catalog_format::Catalog::read(root).unwrap();
        let template = &catalog.templates["browserless"].template;
        let id = Path::new("templates/browserless");
        let raw: Value = serde_saphyr::from_str(
            &fs::read_to_string(root.join(id).join("template.yaml")).unwrap(),
        )
        .unwrap();
        assert_eq!(template_problems(&config, root, id, template, &raw), []);

        let scratch = std::env::temp_dir().join(format!("catalog-readme-{}", std::process::id()));
        let dir = Path::new("browserless");
        fs::create_dir_all(scratch.join(dir)).unwrap();
        for entry in fs::read_dir(root.join(id)).unwrap() {
            let path = entry.unwrap().path();
            fs::copy(&path, scratch.join(dir).join(path.file_name().unwrap())).unwrap();
        }
        fs::write(
            scratch.join(dir).join("README.md"),
            "# Browserless\n\n## Introduction\n\nHeadless Chrome.\n\n## Usage\n\nConnect.\n",
        )
        .unwrap();
        fs::write(
            scratch.join(dir).join("README.zh-TW.md"),
            "## 簡介\n\n無頭 Chrome。\n\n## 開始使用\n\n部署後就能使用。\n\n## 設定\n\n權杖在 Secret browserless-secrets。\n",
        )
        .unwrap();
        let got = template_problems(&config, &scratch, dir, template, &raw);
        fs::remove_dir_all(&scratch).unwrap();
        let english = dir.join("README.md");
        let found = |file: &Path, want: &dyn Fn(&PolicyProblem) -> bool| {
            got.iter().any(|p| {
                p.file == file && matches!(&p.kind, ProblemKind::Policy(kind) if want(kind))
            })
        };
        assert!(
            found(
                &english,
                &|k| matches!(k, PolicyProblem::Level1Heading { line, .. } if line == "# Browserless")
            ),
            "{got:?}"
        );
        assert!(
            found(
                &english,
                &|k| matches!(k, PolicyProblem::Heading { line, .. } if line == "## Usage")
            ),
            "{got:?}"
        );
        assert!(
            found(&english, &|k| *k
                == PolicyProblem::MissingSection {
                    heading: "Getting started".to_owned()
                }),
            "{got:?}"
        );
        assert!(
            found(&dir.join("README.zh-TW.md"), &|k| matches!(
                k,
                PolicyProblem::SecretNotNamed { section, secret, .. }
                    if section == "開始使用" && secret == "browserless-secrets"
            )),
            "{got:?}"
        );
    }
}
