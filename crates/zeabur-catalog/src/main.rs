//! Zeabur Catalog's command. Run it from the repository root.
//!
//! - `zeabur-catalog api --version <v> [--out <dir>]` writes the static API of the release to
//!   `<dir>/v1/` (`out/api` by default) and checks it against its schemas.
//! - `zeabur-catalog compat <published> <api>` fails unless every file of the API in `<api>` passes
//!   the schemas of `<published>`, opened for new fields and values, and its own.
//! - `zeabur-catalog check` prints the problems of the catalog's files and fails if there are any.
//! - `zeabur-catalog render [--out <dir>] [<id>…]` renders each release of each component on each
//!   profile and Kubernetes version into `out/render/` and checks the objects with kubeconform.
//! - `zeabur-catalog digest <dir>…` prints the `sourceDigest` of each component or template
//!   directory.
//! - `zeabur-catalog schema` writes `schema/v1/` and `types/v1/catalog.ts` from the types of
//!   `zeabur-catalog-format`.
//! - `zeabur-catalog licenses-update` reads the SPDX License List's `licenses.json` from stdin and
//!   writes the OSI-approved identifiers to `config/licenses.yaml`.

use std::io::Read;
use std::process::ExitCode;

mod api;
mod check;
mod compat;
mod licenses;
mod lint;
mod policy;
mod render;
mod templates;
mod values;

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let result = match args
        .iter()
        .map(String::as_str)
        .collect::<Vec<_>>()
        .as_slice()
    {
        ["api", "--version", version, rest @ ..] => api(version, rest),
        ["check"] => check(),
        ["compat", published, api] => compat(published, api),
        ["render", rest @ ..] => render(rest),
        ["digest", dirs @ ..] if !dirs.is_empty() => digest(dirs),
        ["schema"] => write(),
        ["licenses-update"] => licenses_update(),
        _ => Err(
            "usage: zeabur-catalog api|check|compat|render|digest|schema|licenses-update"
                .to_owned(),
        ),
    };
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            output::error(&e);
            ExitCode::FAILURE
        }
    }
}

fn api(version: &str, args: &[&str]) -> Result<(), String> {
    let out = match args {
        [] => "out/api",
        ["--out", out] => out,
        _ => return Err("usage: zeabur-catalog api --version <v> [--out <dir>]".to_owned()),
    };
    let dir = std::path::Path::new(out).join(format!("v{}", api::MAJOR));
    let root = std::path::Path::new(".");
    api::write(root, version, &api::commit(root)?, &dir)?;
    let problems = api::check(&dir, &api::Schemas::read(&dir.join("schema"))?)?;
    for problem in &problems {
        output::error(&problem.to_string());
    }
    match problems.len() {
        0 => Ok(()),
        n => Err(format!("{n} problems")),
    }
}

fn check() -> Result<(), String> {
    let problems = check::check(std::path::Path::new("."))?;
    for problem in &problems {
        output::error(&problem.to_string());
    }
    match problems.len() {
        0 => Ok(()),
        n => Err(format!("{n} problems")),
    }
}

fn compat(published: &str, api: &str) -> Result<(), String> {
    let problems = compat::compat(std::path::Path::new(published), std::path::Path::new(api))?;
    for problem in &problems {
        output::error(&problem.to_string());
    }
    match problems.len() {
        0 => Ok(()),
        n => Err(format!("{n} problems")),
    }
}

fn render(args: &[&str]) -> Result<(), String> {
    let (out, ids) = match args {
        ["--out", out, ids @ ..] => (*out, ids),
        ids => ("out/render", ids),
    };
    let ids: Vec<String> = ids.iter().map(|&id| id.to_owned()).collect();
    let problems = render::render(std::path::Path::new("."), &ids, std::path::Path::new(out))?;
    for problem in &problems {
        output::error(&problem.to_string());
    }
    match problems.len() {
        0 => Ok(()),
        n => Err(format!("{n} problems")),
    }
}

fn digest(dirs: &[&str]) -> Result<(), String> {
    for dir in dirs {
        let digest = zeabur_catalog_engine::digest::source_digest(std::path::Path::new(dir))
            .map_err(|e| format!("{dir}: {e}"))?;
        output::print(&digest);
    }
    Ok(())
}

fn write() -> Result<(), String> {
    for (name, text) in zeabur_catalog_format::schema::schemas() {
        let path = format!("schema/v1/{name}");
        std::fs::write(&path, text).map_err(|e| format!("{path}: {e}"))?;
    }
    zeabur_catalog_format::schema::write_typescript(std::path::Path::new("types/v1"))
        .map_err(|e| format!("types/v1/catalog.ts: {e}"))
}

fn licenses_update() -> Result<(), String> {
    const PATH: &str = "config/licenses.yaml";
    let mut list = String::new();
    std::io::stdin()
        .read_to_string(&mut list)
        .map_err(|e| format!("stdin: {e}"))?;
    let current = std::fs::read_to_string(PATH).map_err(|e| format!("{PATH}: {e}"))?;
    std::fs::write(PATH, licenses::update(&current, &list)?).map_err(|e| format!("{PATH}: {e}"))
}

mod output {
    #[expect(clippy::print_stderr, reason = "the output module of the command")]
    pub(crate) fn error(message: &str) {
        eprintln!("zeabur-catalog: {message}");
    }

    #[expect(clippy::print_stdout, reason = "the output module of the command")]
    pub(crate) fn print(line: &str) {
        println!("{line}");
    }

    #[expect(clippy::print_stderr, reason = "the output module of the command")]
    pub(crate) fn info(message: &str) {
        eprintln!("{message}");
    }
}
