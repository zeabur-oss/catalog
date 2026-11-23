//! Zeabur Catalog's command. Run it from the repository root.
//!
//! - `zeabur-catalog check` prints the problems of the catalog's files and fails if there are any.
//! - `zeabur-catalog schema` writes `schema/v1/` and `types/v1/catalog.ts` from the types of
//!   `zeabur-catalog-format`.
//! - `zeabur-catalog licenses-update` reads the SPDX License List's `licenses.json` from stdin and
//!   writes the OSI-approved identifiers to `config/licenses.yaml`.

use std::io::Read;
use std::process::ExitCode;

mod check;
mod licenses;

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let result = match args
        .iter()
        .map(String::as_str)
        .collect::<Vec<_>>()
        .as_slice()
    {
        ["check"] => check(),
        ["schema"] => write(),
        ["licenses-update"] => licenses_update(),
        _ => Err("usage: zeabur-catalog check|schema|licenses-update".to_owned()),
    };
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            output::error(&e);
            ExitCode::FAILURE
        }
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
}
