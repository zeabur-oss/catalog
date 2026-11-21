#![doc = include_str!("../README.md")]

pub mod api;
pub mod common;
pub mod component;
pub mod contributions;
pub mod order;
pub mod select;
pub mod template;
pub mod values;

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

pub use crate::component::Component;
pub use crate::contributions::Contributions;
pub use crate::template::Template;

/// A file of a catalog checkout that does not read.
#[derive(Debug, thiserror::Error)]
pub enum ReadError {
    /// It cannot be read.
    #[error("{path}: {source}")]
    Io {
        /// The file or directory.
        path: PathBuf,
        /// Why.
        source: std::io::Error,
    },
    /// It is no valid file of its kind.
    #[error("{path}: {message}")]
    Format {
        /// The file.
        path: PathBuf,
        /// Why.
        message: String,
    },
    /// A directory holds a file it may not.
    #[error("{path}: {rule}")]
    Layout {
        /// The file.
        path: PathBuf,
        /// What the directory may hold.
        rule: &'static str,
    },
}

/// A template directory: `template.yaml` and the text of `manifests.yaml`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TemplateDir {
    /// `template.yaml`.
    pub template: Template,
    /// `manifests.yaml`, with its template fields.
    pub manifests: String,
}

/// The components and templates of a catalog checkout, by id.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Catalog {
    /// `components/<id>/component.yaml`.
    pub components: BTreeMap<String, Component>,
    /// `templates/<id>/`.
    pub templates: BTreeMap<String, TemplateDir>,
}

impl Catalog {
    /// Reads the checkout at `root`. A component directory holds `component.yaml` and its
    /// READMEs; a template directory holds `template.yaml`, `manifests.yaml`, its READMEs and an
    /// optional `e2e.yaml`.
    ///
    /// # Errors
    ///
    /// A file does not read, is no valid file of its kind, or a directory holds another file.
    pub fn read(root: &Path) -> Result<Self, ReadError> {
        let mut catalog = Self::default();
        for (id, dir) in dirs(&root.join("components"))? {
            check_files(
                &dir,
                &["component.yaml"],
                "a component directory holds component.yaml and its READMEs only",
            )?;
            catalog
                .components
                .insert(id, yaml(&dir.join("component.yaml"))?);
        }
        for (id, dir) in dirs(&root.join("templates"))? {
            check_files(
                &dir,
                &["template.yaml", "manifests.yaml", "e2e.yaml"],
                "a template directory holds template.yaml, manifests.yaml, e2e.yaml and its READMEs only",
            )?;
            let manifests = dir.join("manifests.yaml");
            catalog.templates.insert(
                id,
                TemplateDir {
                    template: yaml(&dir.join("template.yaml"))?,
                    manifests: fs::read_to_string(&manifests).map_err(|source| ReadError::Io {
                        path: manifests,
                        source,
                    })?,
                },
            );
        }
        Ok(catalog)
    }
}

/// Whether `name`, a file in a component or template directory, is one of its READMEs.
#[must_use]
#[expect(
    clippy::case_sensitive_file_extension_comparisons,
    reason = "the catalog names its READMEs README*.md exactly"
)]
pub fn is_readme(name: &str) -> bool {
    name.starts_with("README") && name.ends_with(".md")
}

/// Reads one YAML file strictly into `T`.
///
/// # Errors
///
/// It cannot be read or is no valid `T`.
pub fn yaml<T: serde::de::DeserializeOwned>(path: &Path) -> Result<T, ReadError> {
    let text = fs::read_to_string(path).map_err(|source| ReadError::Io {
        path: path.to_owned(),
        source,
    })?;
    serde_saphyr::from_str(&text).map_err(|e| ReadError::Format {
        path: path.to_owned(),
        message: e.to_string(),
    })
}

/// The subdirectories of `dir` by name; none when `dir` does not exist.
fn dirs(dir: &Path) -> Result<BTreeMap<String, PathBuf>, ReadError> {
    let io = |source| ReadError::Io {
        path: dir.to_owned(),
        source,
    };
    let entries = match fs::read_dir(dir) {
        Ok(entries) => entries,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(BTreeMap::new()),
        Err(e) => return Err(io(e)),
    };
    let mut found = BTreeMap::new();
    for entry in entries {
        let entry = entry.map_err(io)?;
        let path = entry.path();
        if !entry.file_type().map_err(io)?.is_dir() {
            return Err(ReadError::Layout {
                path,
                rule: "each entry is a directory named by its id",
            });
        }
        found.insert(entry.file_name().to_string_lossy().into_owned(), path);
    }
    Ok(found)
}

fn check_files(dir: &Path, allowed: &[&str], rule: &'static str) -> Result<(), ReadError> {
    let io = |source| ReadError::Io {
        path: dir.to_owned(),
        source,
    };
    for entry in fs::read_dir(dir).map_err(io)? {
        let name = entry.map_err(io)?.file_name();
        let name = name.to_string_lossy();
        if !allowed.contains(&name.as_ref()) && !is_readme(&name) {
            return Err(ReadError::Layout {
                path: dir.join(name.as_ref()),
                rule,
            });
        }
    }
    Ok(())
}
