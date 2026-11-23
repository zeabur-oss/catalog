//! Every Helm value a component sets says why: a comment on the value, its key or a block that
//! holds it. Inside a nested block, a comment above its first entry covers the block; at the top
//! of the values each key needs its own.

use granit_parser::{Event, Parser, Placement};
use zeabur_catalog_format::problem::FieldPath;

/// A node of the values, with whether a comment explains it.
#[derive(Debug)]
struct Node {
    /// A comment on the line above it.
    above: bool,
    /// A comment after it on its line.
    right: bool,
    kind: Kind,
}

#[derive(Debug)]
enum Kind {
    Scalar(String),
    Map(Vec<(usize, usize)>),
    Seq(Vec<usize>),
}

impl Node {
    const fn commented(&self) -> bool {
        self.above || self.right
    }
}

/// The paths of the values in `values` that no comment explains, such as `controller.replicas`
/// or `args[0]`.
///
/// # Errors
///
/// `values` is not YAML.
pub(crate) fn unexplained(values: &str) -> Result<Vec<FieldPath>, String> {
    let tree = Tree::parse(values)?;
    let mut out = Vec::new();
    if let Some(root) = tree.root {
        tree.walk(root, &FieldPath::root(), tree.document, &mut out);
    }
    Ok(out)
}

#[derive(Debug, Default)]
struct Tree {
    nodes: Vec<Node>,
    root: Option<usize>,
    /// A comment above the values, apart from the first key, covers all of them.
    document: bool,
}

impl Tree {
    fn parse(text: &str) -> Result<Self, String> {
        let mut tree = Self::default();
        // Each open block, with the key read for its next entry.
        let mut open: Vec<(usize, Option<usize>)> = Vec::new();
        let mut above = false;
        let mut last: Option<usize> = None;
        for next in Parser::new_from_str(text) {
            let (event, _) = next.map_err(|e| e.to_string())?;
            #[expect(
                clippy::wildcard_enum_match_arm,
                reason = "granit-parser's Event and Placement are non-exhaustive"
            )]
            let kind = match event {
                Event::Comment(_, Placement::Above) => {
                    above = true;
                    continue;
                }
                Event::Comment(_, Placement::Right) => {
                    if let Some(node) = last.and_then(|i| tree.nodes.get_mut(i)) {
                        node.right = true;
                    }
                    continue;
                }
                Event::Comment(_, Placement::Free | Placement::Last) => {
                    tree.document |= tree.root.is_none();
                    continue;
                }
                Event::MappingEnd | Event::SequenceEnd => {
                    last = open.pop().map(|(block, _)| block);
                    continue;
                }
                Event::Scalar(value, ..) => Kind::Scalar(value.into_owned()),
                Event::Alias(_) | Event::UnresolvedAlias(_) => Kind::Scalar(String::new()),
                Event::MappingStart(..) => Kind::Map(Vec::new()),
                Event::SequenceStart(..) => Kind::Seq(Vec::new()),
                // Stream and document bounds, anchors, and events of later versions: no node.
                _ => continue,
            };
            let block = matches!(kind, Kind::Map(_) | Kind::Seq(_));
            let index = tree.nodes.len();
            // A comment above a block belongs to its first entry, as YAML parsers attach it.
            tree.nodes.push(Node {
                above: !block && std::mem::take(&mut above),
                right: false,
                kind,
            });
            tree.attach(&mut open, index);
            if block {
                open.push((index, None));
                last = None;
            } else {
                last = Some(index);
            }
        }
        Ok(tree)
    }

    fn attach(&mut self, open: &mut [(usize, Option<usize>)], index: usize) {
        let Some((block, key)) = open.last_mut() else {
            self.root.get_or_insert(index);
            return;
        };
        match self.nodes.get_mut(*block).map(|n| &mut n.kind) {
            Some(Kind::Map(entries)) => match key.take() {
                Some(k) => entries.push((k, index)),
                None => *key = Some(index),
            },
            Some(Kind::Seq(items)) => items.push(index),
            Some(Kind::Scalar(_)) | None => {}
        }
    }

    fn walk(&self, index: usize, path: &FieldPath, explained: bool, out: &mut Vec<FieldPath>) {
        let Some(node) = self.nodes.get(index) else {
            return;
        };
        let explained =
            explained || node.commented() || (!path.is_empty() && self.first_above(node));
        match &node.kind {
            Kind::Map(entries) => {
                for &(key, value) in entries {
                    let key = self.nodes.get(key);
                    let name = match key.map(|k| &k.kind) {
                        Some(Kind::Scalar(name)) => name.as_str(),
                        _ => "",
                    };
                    let by_key = key.is_some_and(Node::commented);
                    self.walk(value, &path.clone().key(name), explained || by_key, out);
                }
            }
            Kind::Seq(items) => {
                for (i, &item) in items.iter().enumerate() {
                    self.walk(item, &path.clone().index(i), explained, out);
                }
            }
            Kind::Scalar(_) => {
                if !explained {
                    out.push(path.clone());
                }
            }
        }
    }

    /// Whether the first entry of a block, or the first key of its first entry, has a comment
    /// above it.
    fn first_above(&self, node: &Node) -> bool {
        let first = match &node.kind {
            Kind::Map(entries) => entries.first().map(|&(key, _)| key),
            Kind::Seq(items) => items.first().copied(),
            Kind::Scalar(_) => None,
        };
        let Some(first) = first.and_then(|i| self.nodes.get(i)) else {
            return false;
        };
        first.above
            || matches!(&first.kind, Kind::Map(entries)
                if entries.first().and_then(|&(k, _)| self.nodes.get(k)).is_some_and(|k| k.above))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn finds_the_values_no_comment_explains() {
        let cases: &[(&str, &[&str])] = &[
            ("", &[]),
            ("a: 1\n", &["a"]),
            ("a: 1 # why\n", &[]),
            ("# why\na: 1\nb: 2\n", &["b"]),
            ("# all of them\n\na: 1\nb: 2\n", &[]),
            ("a: # why\n  b: 1\n  c: 2\n", &[]),
            ("a:\n  # the block\n  b: 1\n  c: 2\n", &[]),
            ("a:\n  b: 1 # only b\n  c: 2\n", &["a.c"]),
            ("a:\n  - x # why\n  - y\n", &["a[1]"]),
            ("a:\n  # the list\n  - name: x\n    value: 1\n", &[]),
            ("a:\n  - name: x\n    # why\n    value: 1\n", &["a[0].name"]),
            ("a: [1, 2] # why\n", &[]),
        ];
        for (values, want) in cases {
            let found: Vec<String> = unexplained(values)
                .unwrap()
                .iter()
                .map(ToString::to_string)
                .collect();
            assert_eq!(found, *want, "{values:?}");
        }
    }

    #[test]
    fn values_must_be_yaml() {
        assert!(unexplained("a: [1\n").is_err());
    }
}
