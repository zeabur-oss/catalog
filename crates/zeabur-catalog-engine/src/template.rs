//! The objects of a template for one namespace.
//!
//! Rendering follows [the catalog's rules for clients][template-clients]. `manifests.yaml` allows
//! field substitution only: this module reads `{{ <field> }}` and refuses every other action.
//!
//! [template-clients]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-clients.mdx

use std::collections::{BTreeMap, BTreeSet};
use std::net::IpAddr;

use regex::Regex;
use serde_json::{Map, Value};
use zeabur_catalog_format::{Template, TemplateDir};

use crate::dns::{is_dns_label, is_hostname};

/// The label that names the instance of a template; `kubectl delete -l` finds every object.
pub const INSTANCE_LABEL: &str = "app.kubernetes.io/instance";
/// The label that names the template.
pub const TEMPLATE_LABEL: &str = "templates.zeabur.com/template";
/// The label that names the template's version.
pub const VERSION_LABEL: &str = "app.kubernetes.io/version";

/// What a deploy gives a template.
#[derive(Debug, Clone, Copy)]
pub struct Options<'a> {
    /// The namespace the objects go in.
    pub namespace: &'a str,
    /// The domain, for a template that routes one.
    pub domain: Option<&'a str>,
    /// The cert-manager `ClusterIssuer`, for a template that asks for certificates.
    pub cluster_issuer: Option<&'a str>,
    /// The values of the inputs that are not generated.
    pub inputs: &'a BTreeMap<String, String>,
}

/// Why a template does not render.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum RenderError {
    /// The namespace is not a DNS label.
    #[error("The namespace {0} is not a DNS label.")]
    Namespace(String),
    /// The template routes a domain and none was given.
    #[error("This template needs a domain.")]
    NoDomain,
    /// The domain is not a DNS name, or it is an IP address.
    #[error("{0} is not a domain name.")]
    Domain(String),
    /// The template asks for certificates and no issuer was given.
    #[error("This template asks cert-manager for certificates, and no ClusterIssuer was given.")]
    NoClusterIssuer,
    /// The issuer name is not a DNS subdomain.
    #[error("{0} is not a ClusterIssuer name.")]
    ClusterIssuer(String),
    /// A generated input was given.
    #[error("The input {0} is generated into a Secret; do not give it.")]
    Generated(String),
    /// A required input is missing.
    #[error("The input {0} is required.")]
    Missing(String),
    /// A value does not match its pattern.
    #[error("The value of {0} does not match its pattern.")]
    Pattern(String),
    /// The template has no such input.
    #[error("The template has no input {0}.")]
    UnknownInput(String),
    /// The template has an action that is not field substitution, or does not read.
    #[error("The template {template} does not read: {detail}")]
    Broken {
        /// The template.
        template: String,
        /// What is wrong.
        detail: String,
    },
}

impl RenderError {
    /// The code that `conformance/template/` gives this error.
    #[must_use]
    pub const fn code(&self) -> &'static str {
        match self {
            Self::Namespace(_) => "invalid-namespace",
            Self::NoDomain => "needs-domain",
            Self::Domain(_) => "invalid-domain",
            Self::NoClusterIssuer => "needs-cluster-issuer",
            Self::ClusterIssuer(_) => "invalid-cluster-issuer",
            Self::Generated(_) => "generated-input",
            Self::Missing(_) => "missing-input",
            Self::Pattern(_) => "pattern",
            Self::UnknownInput(_) => "unknown-input",
            Self::Broken { .. } => "broken",
        }
    }

    /// The input this error names.
    #[must_use]
    pub fn input(&self) -> Option<&str> {
        match self {
            Self::Generated(input)
            | Self::Missing(input)
            | Self::Pattern(input)
            | Self::UnknownInput(input) => Some(input),
            Self::Namespace(_)
            | Self::NoDomain
            | Self::Domain(_)
            | Self::NoClusterIssuer
            | Self::ClusterIssuer(_)
            | Self::Broken { .. } => None,
        }
    }

    /// The template is wrong, not the options.
    #[must_use]
    pub const fn is_internal(&self) -> bool {
        matches!(self, Self::Broken { .. })
    }
}

/// The labels every object of the template `id` gets.
#[must_use]
pub fn labels(id: &str, template: &Template) -> BTreeMap<String, String> {
    [
        (INSTANCE_LABEL, id),
        (TEMPLATE_LABEL, id),
        (VERSION_LABEL, template.metadata.version.as_str()),
    ]
    .into_iter()
    .map(|(k, v)| (k.to_owned(), v.to_owned()))
    .collect()
}

/// Whether the template routes a domain, and so needs a domain and a `ClusterIssuer`.
#[must_use]
pub fn routes_domain(template: &Template) -> bool {
    template.services.iter().any(|s| !s.domains.is_empty())
}

/// Checks what rendering needs of the template `id` itself: `manifests.yaml` has only field
/// substitution, and no two inputs share a name or a Secret key.
///
/// # Errors
///
/// [`RenderError::Broken`] with what is wrong.
pub fn check(id: &str, dir: &TemplateDir) -> Result<(), RenderError> {
    let broken = |detail: String| RenderError::Broken {
        template: id.to_owned(),
        detail,
    };
    parse(&dir.manifests).map_err(broken)?;
    let mut names = BTreeSet::new();
    let mut secrets = BTreeSet::new();
    for input in &dir.template.inputs {
        if !names.insert(input.name.as_str()) {
            return Err(broken(format!("duplicate input {}", input.name)));
        }
        if let Some(secret) = &input.secret
            && !secrets.insert((secret.name.as_str(), secret.key.as_str()))
        {
            return Err(broken(format!(
                "duplicate Secret key {}/{}",
                secret.name, secret.key
            )));
        }
    }
    Ok(())
}

/// The objects of the template `id` for `options`, in the order of `manifests.yaml`, each with
/// the template's [`labels`].
///
/// # Errors
///
/// The options do not fit the template, or the template is [broken](RenderError::Broken).
pub fn render(
    id: &str,
    dir: &TemplateDir,
    options: &Options<'_>,
) -> Result<Vec<Value>, RenderError> {
    let broken = |detail: String| RenderError::Broken {
        template: id.to_owned(),
        detail,
    };
    check(id, dir)?;
    check_options(id, &dir.template, options)?;
    let mut inputs: BTreeMap<&str, &str> = options
        .inputs
        .iter()
        .map(|(k, v)| (k.as_str(), v.as_str()))
        .collect();
    // An input that is not generated and not given takes its default, or is empty.
    for input in dir.template.inputs.iter().filter(|i| i.generate.is_none()) {
        inputs
            .entry(input.name.as_str())
            .or_insert_with(|| input.default.as_deref().unwrap_or_default());
    }
    let values = Values {
        namespace: options.namespace,
        domain: options.domain.unwrap_or_default(),
        cluster_issuer: options.cluster_issuer.unwrap_or_default(),
        inputs,
    };
    let nodes = parse(&dir.manifests).map_err(broken)?;
    let mut text = String::with_capacity(dir.manifests.len());
    execute(&nodes, &values, &mut text).map_err(broken)?;
    let documents: Vec<Value> =
        serde_saphyr::from_multiple(&text).map_err(|e| broken(e.to_string()))?;
    let labels = labels(id, &dir.template);
    let mut objects: Vec<Value> = documents.into_iter().filter(|o| !o.is_null()).collect();
    for object in &mut objects {
        let Some(object_labels) = object
            .as_object_mut()
            .map(|o| child(o, "metadata"))
            .and_then(Value::as_object_mut)
            .map(|m| child(m, "labels"))
            .and_then(Value::as_object_mut)
        else {
            return Err(broken("an object without metadata labels".to_owned()));
        };
        for (k, v) in &labels {
            object_labels.insert(k.clone(), Value::from(v.as_str()));
        }
    }
    Ok(objects)
}

/// The value of `key`, an empty object when `object` lacks it.
fn child<'a>(object: &'a mut Map<String, Value>, key: &str) -> &'a mut Value {
    object
        .entry(key)
        .or_insert_with(|| Value::Object(Map::new()))
}

fn check_options(id: &str, template: &Template, options: &Options<'_>) -> Result<(), RenderError> {
    if !is_dns_label(options.namespace) {
        return Err(RenderError::Namespace(options.namespace.to_owned()));
    }
    let routes_domain = routes_domain(template);
    match options.domain.filter(|d| !d.is_empty()) {
        Some(domain) if !is_hostname(domain) || domain.parse::<IpAddr>().is_ok() => {
            return Err(RenderError::Domain(domain.to_owned()));
        }
        None if routes_domain => return Err(RenderError::NoDomain),
        Some(_) | None => {}
    }
    match options.cluster_issuer.filter(|i| !i.is_empty()) {
        Some(issuer) if !is_hostname(issuer) => {
            return Err(RenderError::ClusterIssuer(issuer.to_owned()));
        }
        None if routes_domain => return Err(RenderError::NoClusterIssuer),
        Some(_) | None => {}
    }
    for input in &template.inputs {
        let value = options.inputs.get(&input.name);
        if input.generate.is_some() {
            if value.is_some() {
                return Err(RenderError::Generated(input.name.clone()));
            }
            continue;
        }
        match value {
            None if input.required => return Err(RenderError::Missing(input.name.clone())),
            None => {}
            Some(value) => {
                let pattern = Regex::new(&format!(r"\A(?:{})\z", re2(&input.validation.pattern)))
                    .map_err(|e| RenderError::Broken {
                    template: id.to_owned(),
                    detail: format!("the pattern of {}: {e}", input.name),
                })?;
                if !pattern.is_match(value) {
                    return Err(RenderError::Pattern(input.name.clone()));
                }
            }
        }
    }
    if let Some(unknown) = options
        .inputs
        .keys()
        .find(|k| !template.inputs.iter().any(|i| &i.name == *k))
    {
        return Err(RenderError::UnknownInput(unknown.clone()));
    }
    Ok(())
}

/// `pattern` with the Perl classes and word boundaries of [RE2], which are ASCII, where the
/// `regex` crate's are Unicode. Each class becomes a bracket class, which is also valid inside
/// another class.
///
/// [RE2]: https://github.com/google/re2/wiki/Syntax
fn re2(pattern: &str) -> String {
    let mut out = String::with_capacity(pattern.len());
    let mut chars = pattern.chars();
    while let Some(c) = chars.next() {
        if c != '\\' {
            out.push(c);
            continue;
        }
        let Some(next) = chars.next() else {
            out.push(c);
            break;
        };
        match next {
            'd' => out.push_str("[0-9]"),
            'D' => out.push_str("[^0-9]"),
            's' => out.push_str("[\\t\\n\\f\\r ]"),
            'S' => out.push_str("[^\\t\\n\\f\\r ]"),
            'w' => out.push_str("[0-9A-Za-z_]"),
            'W' => out.push_str("[^0-9A-Za-z_]"),
            'b' => out.push_str("(?-u:\\b)"),
            'B' => out.push_str("(?-u:\\B)"),
            _ => {
                out.push(c);
                out.push(next);
            }
        }
    }
    out
}

/// A field of the data a template reads.
#[derive(Debug, Clone, PartialEq, Eq)]
enum Field {
    Namespace,
    Domain,
    ClusterIssuer,
    Input(String),
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum Node {
    Text(String),
    Field(Field),
}

fn parse(text: &str) -> Result<Vec<Node>, String> {
    let mut nodes = Vec::new();
    let mut rest = text;
    while let Some((before, after)) = rest.split_once("{{") {
        let (inner, next) = after
            .split_once("}}")
            .ok_or_else(|| "an action has no closing }}".to_owned())?;
        if !before.is_empty() {
            nodes.push(Node::Text(before.to_owned()));
        }
        nodes.push(Node::Field(field(inner.trim_matches(is_go_space))?));
        rest = next;
    }
    if !rest.is_empty() {
        nodes.push(Node::Text(rest.to_owned()));
    }
    Ok(nodes)
}

/// The white space that Go's `text/template` skips around an action.
const fn is_go_space(c: char) -> bool {
    matches!(c, ' ' | '\t' | '\r' | '\n')
}

fn field(name: &str) -> Result<Field, String> {
    match name {
        ".Namespace" => Ok(Field::Namespace),
        ".Domain" => Ok(Field::Domain),
        ".ClusterIssuer" => Ok(Field::ClusterIssuer),
        _ => name
            .strip_prefix(".Inputs.")
            .filter(|n| is_input_name(n))
            .map(|n| Field::Input(n.to_owned()))
            .ok_or_else(|| format!("a template does not run the action {{{{ {name} }}}}")),
    }
}

/// An input name: `^[A-Za-z][A-Za-z0-9_]*$`.
fn is_input_name(name: &str) -> bool {
    let mut bytes = name.bytes();
    bytes.next().is_some_and(|b| b.is_ascii_alphabetic())
        && bytes.all(|b| b.is_ascii_alphanumeric() || b == b'_')
}

struct Values<'a> {
    namespace: &'a str,
    domain: &'a str,
    cluster_issuer: &'a str,
    inputs: BTreeMap<&'a str, &'a str>,
}

impl Values<'_> {
    fn get(&self, field: &Field) -> Result<&str, String> {
        match field {
            Field::Namespace => Ok(self.namespace),
            Field::Domain => Ok(self.domain),
            Field::ClusterIssuer => Ok(self.cluster_issuer),
            // Go's `missingkey=error`: an input the template does not declare, or a generated one.
            Field::Input(name) => self
                .inputs
                .get(name.as_str())
                .copied()
                .ok_or_else(|| format!("it reads the input {name}, which has no value")),
        }
    }
}

fn execute(nodes: &[Node], values: &Values<'_>, out: &mut String) -> Result<(), String> {
    for node in nodes {
        match node {
            Node::Text(text) => out.push_str(text),
            Node::Field(field) => out.push_str(values.get(field)?),
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn perl_classes_and_boundaries_are_ascii_as_in_re2() {
        for (pattern, value, matches) in [
            (r"\d+", "12", true),
            (r"\d+", "\u{661}\u{662}", false),
            (r"\w+", "caf\u{e9}", false),
            (r"[^@\s]+", "a\u{a0}b", true),
            (r"\S+", "a\u{a0}b", true),
            (r"[\d\-]+", "1-2", true),
            (r"\\d", "\\d", true),
            (r".*\bx", "\u{e9}x", true),
        ] {
            let regex = Regex::new(&format!(r"\A(?:{})\z", re2(pattern))).unwrap();
            assert_eq!(regex.is_match(value), matches, "{pattern} {value:?}");
        }
    }

    fn run(text: &str, inputs: &[(&str, &str)]) -> Result<String, String> {
        let values = Values {
            namespace: "shop",
            domain: "app.example.com",
            cluster_issuer: "letsencrypt",
            inputs: inputs.iter().copied().collect(),
        };
        let mut out = String::new();
        execute(&parse(text)?, &values, &mut out)?;
        Ok(out)
    }

    /// A name, a template, its inputs and what it renders to.
    type Case<'a> = (&'a str, &'a str, &'a [(&'a str, &'a str)], &'a str);

    #[test]
    fn substitutes_fields() {
        let cases: &[Case<'_>] = &[
            (
                "fields",
                "ns: {{ .Namespace }} host: {{ .Domain }} by: {{ .ClusterIssuer }}",
                &[],
                "ns: shop host: app.example.com by: letsencrypt",
            ),
            (
                "input",
                "port: '{{ .Inputs.port }}'",
                &[("port", "465")],
                "port: '465'",
            ),
            (
                "input with an underscore",
                "{{.Inputs.smtp_port}}",
                &[("smtp_port", "587")],
                "587",
            ),
            ("text", "a -}} b {x}", &[], "a -}} b {x}"),
        ];
        for (name, text, inputs, expected) in cases {
            assert_eq!(run(text, inputs).as_deref(), Ok(*expected), "{name}");
        }
    }

    #[test]
    fn refuses_what_it_does_not_run() {
        for text in [
            "{{ .Values.x }}",
            "{{ printf \"%s\" .Namespace }}",
            "{{ range .Inputs }}x{{ end }}",
            "{{ if .Inputs.a }}x{{ end }}",
            "{{ eq .Inputs.a \"a\" }}",
            "{{ dnsEgress }}",
            "{{- .Namespace }}",
            "{{ .Namespace -}}",
            "{{ .Namespace | printf }}",
            "{{ .Namespace",
            "{{ .Inputs.unset }}",
        ] {
            assert!(run(text, &[]).is_err(), "{text}");
        }
    }
}
