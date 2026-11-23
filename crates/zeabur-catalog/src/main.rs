//! `zeabur-catalog schema` writes `schema/v1/` and `types/v1/catalog.ts` from the types of
//! `zeabur-catalog-format`. Run it from the repository root.

use std::process::ExitCode;

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let result = match args
        .iter()
        .map(String::as_str)
        .collect::<Vec<_>>()
        .as_slice()
    {
        ["schema"] => write(),
        _ => Err("usage: zeabur-catalog schema".to_owned()),
    };
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            output::error(&e);
            ExitCode::FAILURE
        }
    }
}

fn write() -> Result<(), String> {
    let schemas = zeabur_catalog_format::schema::schemas().map_err(|e| e.to_string())?;
    for (name, text) in schemas {
        let path = format!("schema/v1/{name}");
        std::fs::write(&path, text).map_err(|e| format!("{path}: {e}"))?;
    }
    zeabur_catalog_format::schema::write_typescript(std::path::Path::new("types/v1"))
        .map_err(|e| format!("types/v1/catalog.ts: {e}"))
}

mod output {
    #[expect(clippy::print_stderr, reason = "the output module of the command")]
    pub(crate) fn error(message: &str) {
        eprintln!("zeabur-catalog: {message}");
    }
}
