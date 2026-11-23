//! The objects of a template for one namespace.
//!
//! Rendering follows [the catalog's rules for clients][template-clients]. `manifests.yaml` allows
//! field substitution only: this module reads `{{ <field> }}` and refuses every other action.
//!
//! [template-clients]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-clients.mdx

use std::collections::{BTreeMap, BTreeSet};
use std::net::IpAddr;

use regex::Regex;
use regex_syntax::ast::{
    self, AssertionKind, Ast, ClassSet, ClassSetItem, GroupKind, Literal, LiteralKind, Span,
};
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
        if let Some(problem) = pattern_problem(&input.validation.pattern) {
            return Err(broken(format!("the pattern of {}: {problem}", input.name)));
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
#[expect(
    clippy::disallowed_types,
    clippy::disallowed_methods,
    reason = "a template renders Kubernetes objects of any kind, custom resources included"
)]
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
#[expect(
    clippy::disallowed_types,
    reason = "a part of a rendered object, which may be of any kind"
)]
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
                let pattern =
                    whole_match(&input.validation.pattern).map_err(|e| RenderError::Broken {
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

/// `pattern` matching a whole value.
fn whole_match(pattern: &str) -> Result<Regex, regex::Error> {
    Regex::new(&format!(r"\A(?:{pattern})\z"))
}

/// Why `pattern` is outside [the syntax every client reads the same way][template-clients]:
/// ASCII literals, `.`, bracket classes of literals and ranges, groups, `|`, repetition, `^` and
/// `$`. RE2, the `regex` crate and JavaScript differ on the rest, such as `\d`, `\s`, `\b`,
/// `\p{..}`, flags and nested classes.
///
/// [template-clients]: https://github.com/zeabur-oss/catalog/blob/main/docs/reference/template-clients.mdx
fn pattern_problem(pattern: &str) -> Option<String> {
    if !pattern.is_ascii() {
        return Some("it has a character that is not ASCII".to_owned());
    }
    let ast = match ast::parse::Parser::new().parse(pattern) {
        Ok(ast) => ast,
        Err(e) => return Some(e.to_string()),
    };
    let span = portable(&ast).err()?;
    let text = pattern
        .get(span.start.offset..span.end.offset)
        .unwrap_or(pattern);
    Some(format!("{text} is not read the same way by every client"))
}

fn portable(ast: &Ast) -> Result<(), Span> {
    match ast {
        Ast::Empty(_) | Ast::Dot(_) => Ok(()),
        Ast::Literal(literal) => portable_literal(literal),
        Ast::Assertion(assertion) => match assertion.kind {
            AssertionKind::StartLine | AssertionKind::EndLine => Ok(()),
            AssertionKind::StartText
            | AssertionKind::EndText
            | AssertionKind::WordBoundary
            | AssertionKind::NotWordBoundary
            | AssertionKind::WordBoundaryStart
            | AssertionKind::WordBoundaryEnd
            | AssertionKind::WordBoundaryStartAngle
            | AssertionKind::WordBoundaryEndAngle
            | AssertionKind::WordBoundaryStartHalf
            | AssertionKind::WordBoundaryEndHalf => Err(assertion.span),
        },
        Ast::ClassBracketed(class) => match &class.kind {
            ClassSet::Item(item) => portable_item(item),
            ClassSet::BinaryOp(op) => Err(op.span),
        },
        Ast::Repetition(repetition) => portable(&repetition.ast),
        Ast::Group(group) => match &group.kind {
            GroupKind::CaptureIndex(_) => portable(&group.ast),
            GroupKind::NonCapturing(flags) if flags.items.is_empty() => portable(&group.ast),
            GroupKind::NonCapturing(_) | GroupKind::CaptureName { .. } => Err(group.span),
        },
        Ast::Alternation(alternation) => alternation.asts.iter().try_for_each(portable),
        Ast::Concat(concat) => concat.asts.iter().try_for_each(portable),
        Ast::Flags(flags) => Err(flags.span),
        Ast::ClassUnicode(class) => Err(class.span),
        Ast::ClassPerl(class) => Err(class.span),
    }
}

/// A member of a bracket class. A `]` is escaped: JavaScript reads `[]` as an empty class.
fn portable_item(item: &ClassSetItem) -> Result<(), Span> {
    match item {
        ClassSetItem::Literal(literal)
            if literal.kind == LiteralKind::Verbatim && literal.c == ']' =>
        {
            Err(literal.span)
        }
        ClassSetItem::Literal(literal) => portable_literal(literal),
        ClassSetItem::Range(range) => {
            portable_literal(&range.start)?;
            portable_literal(&range.end)
        }
        ClassSetItem::Union(union) => union.items.iter().try_for_each(portable_item),
        ClassSetItem::Empty(span) => Err(*span),
        ClassSetItem::Ascii(class) => Err(class.span),
        ClassSetItem::Unicode(class) => Err(class.span),
        ClassSetItem::Perl(class) => Err(class.span),
        ClassSetItem::Bracketed(class) => Err(class.span),
    }
}

/// A character as itself, or a special character escaped.
const fn portable_literal(literal: &Literal) -> Result<(), Span> {
    match literal.kind {
        LiteralKind::Verbatim | LiteralKind::Meta | LiteralKind::Superfluous => Ok(()),
        LiteralKind::Octal
        | LiteralKind::HexFixed(_)
        | LiteralKind::HexBrace(_)
        | LiteralKind::Special(_) => Err(literal.span),
    }
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
    fn a_pattern_keeps_to_what_every_client_reads_the_same_way() {
        for pattern in [
            "^[A-Za-z0-9_-]{43}$",
            r"^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$",
            "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
            "^(?:465|587)$",
            r"^https://[A-Za-z0-9.-]+(:443)?(/[A-Za-z0-9._~/-]*)?$",
            "[^@ ]+@[^@ ]+",
            r"[\]a]+",
            "a|",
            "x{2,}?",
        ] {
            assert_eq!(pattern_problem(pattern), None, "{pattern}");
        }
        for (pattern, at) in [
            (r"\d+", r"\d"),
            (r"[^@\s]+", r"\s"),
            (r"\w", r"\w"),
            (r".*\bx", r"\b"),
            (r"\p{Greek}", r"\p{Greek}"),
            ("(?i)https", "(?i)"),
            ("(?i:a)", "(?i:a)"),
            ("(?P<n>a)", "(?P<n>a)"),
            (r"\Aa\z", r"\A"),
            (r"\x61", r"\x61"),
            (r"\t", r"\t"),
            ("[[:digit:]]", "[:digit:]"),
            ("[a[b]]", "[b]"),
            ("[a&&b]", "a&&b"),
            ("[]a]", "]"),
            ("caf\u{e9}", "it has a character that is not ASCII"),
        ] {
            let problem = pattern_problem(pattern).unwrap_or_default();
            assert!(problem.starts_with(at), "{pattern}: {problem}");
        }
        for unread in ["a{", "a{,3}", r"\141", r"\1"] {
            assert!(pattern_problem(unread).is_some(), "{unread}");
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
