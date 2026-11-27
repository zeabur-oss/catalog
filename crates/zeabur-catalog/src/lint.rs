//! The template lint: the rules for the objects a template renders into. They stay in the
//! template's namespace, run pinned images with bounded resources at the Pod Security level of
//! their service, reach only the pods and networks they declare, and match the services of
//! `template.yaml`.
//!
//! The objects of an operator are custom resources, which the lint reads as the operator's
//! fields it knows. A field of another shape reads as empty, so a rule that needs it fails.

mod operators;
mod safety;
mod services;
mod view;

#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
use serde_json::Value;
use zeabur_catalog_format::common::Reference;
use zeabur_catalog_format::problem::{LintProblem, LintRule};
use zeabur_catalog_format::template::Template;

/// What the lint checks: a template, rendered into a namespace.
#[derive(Debug, Clone, Copy)]
pub(crate) struct Subject<'a> {
    /// The template ID.
    pub(crate) id: &'a str,
    /// `template.yaml`.
    pub(crate) template: &'a Template,
    /// The namespace the objects are rendered for.
    pub(crate) namespace: &'a str,
}

impl Subject<'_> {
    /// Whether the template requires the component `id` by its ID.
    fn requires(&self, id: &str) -> bool {
        self.template
            .requires
            .iter()
            .any(|r| matches!(r, Reference::Component(c) if c == id))
    }

    /// Whether `security.risks` lists a risk.
    const fn has_risks(&self) -> bool {
        !self.template.security.risks.is_empty()
    }
}

/// The problems the lint finds in `docs`, the objects `subject` renders into.
#[expect(
    clippy::disallowed_types,
    reason = "a template renders objects of any kind, custom resources included"
)]
pub(crate) fn lint(subject: &Subject<'_>, docs: &[Value]) -> Vec<LintProblem> {
    let mut found = Found::default();
    safety::check(subject, docs, &mut found);
    services::check(subject.template, docs, &mut found);
    found.0
}

/// The problems found so far, in order.
#[derive(Debug, Default)]
struct Found(Vec<LintProblem>);

impl Found {
    /// Adds `rule`, broken at `at`.
    fn push(&mut self, at: &str, rule: LintRule) {
        self.0.push(LintProblem {
            at: at.to_owned(),
            rule,
        });
    }
}

#[cfg(test)]
mod tests {
    #[expect(
        clippy::disallowed_types,
        reason = "a template renders objects of any kind"
    )]
    use serde_json::Value;
    use zeabur_catalog_format::common::Reference;
    use zeabur_catalog_format::problem::LintRule;
    use zeabur_catalog_format::template::{Service, Template};

    use super::{Found, Subject};

    /// A Deployment that breaks no rule for the template `app`.
    pub(super) const SAFE: &str = "apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  template:
    metadata:
      labels: {app: app, templates.zeabur.com/template: app}
    spec:
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: app
          image: example.test/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
          securityContext:
            allowPrivilegeEscalation: false
            capabilities: {drop: [ALL]}
          resources:
            requests: {cpu: 10m, memory: 16Mi}
            limits: {cpu: 100m, memory: 32Mi}
          readinessProbe: {tcpSocket: {port: 8080}}
          livenessProbe: {tcpSocket: {port: 8080}}
";

    /// `text` with the first `from` replaced; panics when `text` has no `from`.
    pub(super) fn replace(text: &str, from: &str, to: &str) -> String {
        assert!(text.contains(from), "{text} has no {from}");
        text.replacen(from, to, 1)
    }

    /// `@sha256:` and a digest.
    pub(super) fn digest() -> String {
        format!("@sha256:{}", "a".repeat(64))
    }

    /// A NetworkPolicy that allows ingress to the template's pods from `peer`.
    pub(super) fn allow(peer: &str) -> String {
        format!(
            "---\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata: {{name: allow}}\n\
             spec:\n  podSelector: {{matchLabels: {{templates.zeabur.com/template: app}}}}\n  \
             policyTypes: [Ingress]\n  ingress: [{{from: [{peer}]}}]\n"
        )
    }

    /// A template with the services `(name, podSecurity)`, which requires the components
    /// `requires`.
    pub(super) fn template(services: &[(&str, &str)], requires: &[&str]) -> Template {
        let mut template: Template = serde_saphyr::from_str(
            "apiVersion: catalog.zeabur.com/v1
kind: Template
metadata:
  name: App
  summary: An app.
  locales: {}
  license: MIT
  homepage: https://example.test
  docs: https://example.test
  version: '1'
services: []
security: {summary: An app., risks: []}
smoke: []
",
        )
        .unwrap();
        template.services = services
            .iter()
            .map(|(name, level)| service(name, level))
            .collect();
        template.requires = requires
            .iter()
            .map(|id| Reference::Component((*id).to_owned()))
            .collect();
        template
    }

    /// A service with `podSecurity` `level` that requests and limits `1m` and `1Mi`.
    pub(super) fn service(name: &str, level: &str) -> Service {
        serde_saphyr::from_str(&format!(
            "{{name: {name}, podSecurity: {level}, resources: {{requests: {{cpu: 1m, memory: \
             1Mi}}, limits: {{cpu: 1m, memory: 1Mi}}}}}}"
        ))
        .unwrap()
    }

    /// Adds a risk to `security.risks`.
    pub(super) fn with_risk(mut template: Template) -> Template {
        template.security.risks = serde_saphyr::from_str(
            "[{risk: Operator generated pods, mitigation: Pinned catalog review, locales: {}}]",
        )
        .unwrap();
        template
    }

    #[expect(
        clippy::disallowed_types,
        reason = "a template renders objects of any kind"
    )]
    pub(super) fn docs(manifest: &str) -> Vec<Value> {
        crate::render::documents(manifest.as_bytes()).unwrap()
    }

    /// The template `app` in the namespace `test`.
    pub(super) const fn subject(template: &Template) -> Subject<'_> {
        Subject {
            id: "app",
            template,
            namespace: "test",
        }
    }

    /// The rules the objects of `manifest` break of the rules for every object, the operator
    /// rules included.
    pub(super) fn safety(template: &Template, manifest: &str) -> Vec<LintRule> {
        let mut found = Found::default();
        super::safety::check(&subject(template), &docs(manifest), &mut found);
        found.0.into_iter().map(|p| p.rule).collect()
    }

    /// Asserts that `rules` has a rule that matches the pattern.
    macro_rules! assert_breaks {
        ($rules:expr, $pattern:pat $(if $guard:expr)?) => {{
            let rules = &$rules;
            assert!(
                rules.iter().any(|r| matches!(r, $pattern $(if $guard)?)),
                "{rules:?} has no {}",
                stringify!($pattern)
            );
        }};
    }
    pub(super) use assert_breaks;
}
