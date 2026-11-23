//! The source digest of a component or template directory: Go's
//! [`dirhash.Hash1`](https://pkg.go.dev/golang.org/x/mod/sumdb/dirhash#Hash1), the `h1:` hash of
//! `go.sum`, over every file of the directory except `e2e.yaml` and the READMEs.

use std::fmt::Write as _;
use std::path::{Path, PathBuf};
use std::{fs, io};

use base64::Engine as _;
use base64::engine::general_purpose::STANDARD;
use sha2::{Digest as _, Sha256};

/// The `h1:` digest of `dir`. A measurement with another digest tested other files.
///
/// # Errors
///
/// Fails when the directory or one of its files cannot be read, or a file name has a newline.
pub fn source_digest(dir: &Path) -> io::Result<String> {
    let mut files = Vec::new();
    list(dir, dir, &mut files)?;
    files.retain(|(name, _)| !skipped(name));
    files.sort();
    hash1(
        files
            .iter()
            .map(|(name, path)| Ok((name.as_str(), fs::read(path)?))),
    )
}

/// A file that does not change what an install does.
fn skipped(name: &str) -> bool {
    name == "e2e.yaml" || zeabur_catalog_format::is_readme(name)
}

/// Every file under `dir`, named by its slash-separated path from `root`, as `DirFiles` does.
fn list(root: &Path, dir: &Path, files: &mut Vec<(String, PathBuf)>) -> io::Result<()> {
    for entry in fs::read_dir(dir)? {
        let path = entry?.path();
        if path.is_dir() {
            list(root, &path, files)?;
        } else {
            let name = path
                .strip_prefix(root)
                .map_err(io::Error::other)?
                .components()
                .map(|c| c.as_os_str().to_string_lossy())
                .collect::<Vec<_>>()
                .join("/");
            files.push((name, path));
        }
    }
    Ok(())
}

/// `Hash1` of files sorted by name: the SHA-256 of one `<sha256 hex>  <name>\n` line per file.
fn hash1<'a>(files: impl Iterator<Item = io::Result<(&'a str, Vec<u8>)>>) -> io::Result<String> {
    let mut summary = String::new();
    for file in files {
        let (name, data) = file?;
        if name.contains('\n') {
            return Err(io::Error::other(format!(
                "{name:?}: a file name has a newline"
            )));
        }
        for byte in Sha256::digest(&data) {
            write!(summary, "{byte:02x}").map_err(io::Error::other)?;
        }
        writeln!(summary, "  {name}").map_err(io::Error::other)?;
    }
    Ok(format!("h1:{}", STANDARD.encode(Sha256::digest(summary))))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hashes_the_files_as_go_does() {
        let dir = std::env::temp_dir().join(format!("source-digest-{}", std::process::id()));
        fs::create_dir_all(dir.join("files")).unwrap();
        fs::write(dir.join("component.yaml"), "id: web\n").unwrap();
        fs::write(dir.join("files/values.yaml"), "a: 1\n").unwrap();
        fs::write(dir.join("README.md"), "# Web\n").unwrap();
        fs::write(dir.join("README.zh-TW.md"), "# Web\n").unwrap();
        fs::write(dir.join("e2e.yaml"), "smoke: []\n").unwrap();
        let digest = source_digest(&dir);
        fs::remove_dir_all(&dir).unwrap();
        assert_eq!(
            digest.unwrap(),
            "h1:i39eBL6TwfFf7ooUFLl+FJ6ZHJ2X4ej5hVKNXm00054="
        );
    }
}
