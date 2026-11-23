//! The rules for a template in this repository, on top of what rendering needs: its ID, its
//! services, the image its version names and what it requires.

use std::collections::BTreeSet;

use regex::Regex;
use zeabur_catalog_format::common::Profile;
use zeabur_catalog_format::problem::{FieldPath, Problems, TemplateProblem};
use zeabur_catalog_format::{Catalog, TemplateDir, order};

use crate::check::is_id;

/// Adds the problems of the template `id`, whose `template.yaml` reads `text`.
pub(crate) fn problems(
    catalog: &Catalog,
    id: &str,
    dir: &TemplateDir,
    text: &str,
    found: &mut Problems,
) {
    let template = &dir.template;
    // The e2e namespace `templates-e2e-<id>` must be a DNS label.
    if !is_id(id) || id.len() > 49 {
        found.push(FieldPath::root(), TemplateProblem::Id);
    }
    if catalog.components.contains_key(id) {
        found.push(FieldPath::root(), TemplateProblem::SameIdAsComponent);
    }
    let mut names = BTreeSet::new();
    for (i, service) in template.services.iter().enumerate() {
        if !names.insert(service.name.as_str()) {
            let name = service.name.clone();
            let at = FieldPath::root().key("services").index(i).key("name");
            found.push(at, TemplateProblem::DuplicateService { name });
        }
    }
    if let Err(problem) = version(text, &dir.manifests, &template.metadata.version) {
        found.push(FieldPath::root().key("metadata").key("version"), problem);
    }
    for profile in Profile::ALL {
        let order = template.requires.iter().try_for_each(|reference| {
            let provider = order::provider(&catalog.components, reference, profile, id)?;
            order::install_order(&catalog.components, provider, profile).map(drop)
        });
        if let Err(error) = order {
            let at = FieldPath::root().key("requires");
            found.push(at, TemplateProblem::Requires { profile, error });
        }
    }
}

/// Checks that `metadata.version` names, on the line above it, an image of the manifests pinned at
/// that tag with a digest, so that Renovate moves the version with the image.
fn version(text: &str, manifests: &str, version: &str) -> Result<(), TemplateProblem> {
    // The pattern is a constant that compiles; were it not to, no version would match.
    let above = Regex::new(r#"(?m)^  # renovate: depName=(\S+)\n  version: "?([^"\s]+)"?$"#).ok();
    let image = above
        .and_then(|above| {
            above
                .captures(text)
                .filter(|c| c.get(2).is_some_and(|v| v.as_str() == version))
                .and_then(|c| c.get(1))
                .map(|m| m.as_str().to_owned())
        })
        .ok_or(TemplateProblem::VersionWithoutImage)?;
    // An escaped literal compiles; were it not to, the version would not count as pinned.
    let pinned = Regex::new(&format!(
        r#"(^|[\s"']){}"#,
        regex::escape(&format!("{image}:{version}@sha256:"))
    ))
    .ok();
    if is_label_value(version) && pinned.is_some_and(|p| p.is_match(manifests)) {
        Ok(())
    } else {
        Err(TemplateProblem::VersionNotPinned { image })
    }
}

/// Whether `value` is a Kubernetes label value [label]: at most 63 characters, alphanumerics with
/// `-`, `_` and `.` inside. Templates label every object with their version.
///
/// [label]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
fn is_label_value(value: &str) -> bool {
    let inner = |b: u8| b.is_ascii_alphanumeric() || matches!(b, b'-' | b'_' | b'.');
    !value.is_empty()
        && value.len() <= 63
        && value.bytes().all(inner)
        && value
            .bytes()
            .next()
            .is_some_and(|b| b.is_ascii_alphanumeric())
        && value
            .bytes()
            .last()
            .is_some_and(|b| b.is_ascii_alphanumeric())
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use zeabur_catalog_format::problem::ProblemKind;

    use super::*;

    const TEXT: &str = "metadata:\n  # renovate: depName=ghcr.io/a/b\n  version: \"1.2\"\n";

    #[test]
    fn the_version_is_the_pinned_tag_of_the_named_image() {
        let pinned = "image: ghcr.io/a/b:1.2@sha256:00";
        let not_pinned = Err(TemplateProblem::VersionNotPinned {
            image: "ghcr.io/a/b".to_owned(),
        });
        assert_eq!(version(TEXT, pinned, "1.2"), Ok(()));
        assert_eq!(version(TEXT, "image: ghcr.io/a/b:1.2", "1.2"), not_pinned);
        assert_eq!(
            version(TEXT, "image: ghcr.io/a/bc:1.2@sha256:00", "1.2"),
            not_pinned
        );
        assert_eq!(
            version(TEXT, pinned, "1.3"),
            Err(TemplateProblem::VersionWithoutImage)
        );
        assert_eq!(
            version("metadata:\n  version: \"1.2\"\n", pinned, "1.2"),
            Err(TemplateProblem::VersionWithoutImage)
        );
    }

    #[test]
    fn a_template_needs_its_own_dns_label_and_unique_services() {
        let root = Path::new(concat!(env!("CARGO_MANIFEST_DIR"), "/../.."));
        let catalog = Catalog::read(root).unwrap();
        let mut dir = catalog.templates["miniflux"].clone();
        let text = std::fs::read_to_string(root.join("templates/miniflux/template.yaml")).unwrap();
        let found = |id: &str, dir: &TemplateDir| {
            let mut found = Problems::new(Path::new("template.yaml"));
            problems(&catalog, id, dir, &text, &mut found);
            found
                .into_vec()
                .into_iter()
                .map(|p| (p.at, p.kind))
                .collect::<Vec<_>>()
        };
        let kind = |p: TemplateProblem| ProblemKind::from(p);
        assert_eq!(found("miniflux", &dir), []);
        assert_eq!(
            found("traefik", &dir),
            [(FieldPath::root(), kind(TemplateProblem::SameIdAsComponent))]
        );
        assert_eq!(
            found(&"a".repeat(50), &dir),
            [(FieldPath::root(), kind(TemplateProblem::Id))]
        );
        let service = dir.template.services[0].clone();
        dir.template.services.push(service.clone());
        let at = FieldPath::root()
            .key("services")
            .index(dir.template.services.len() - 1)
            .key("name");
        let name = service.name;
        assert_eq!(
            found("miniflux", &dir),
            [(at, kind(TemplateProblem::DuplicateService { name }))]
        );
    }
}
