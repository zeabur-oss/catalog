//! Zeabur Catalog's command. Run it from the repository root.
//!
//! - `zeabur-catalog api --version <v> [--out <dir>]` writes the static API of the release to
//!   `<dir>/v1/` (`out/api` by default) and checks it against its schemas.
//! - `zeabur-catalog compat <published> <api>` fails unless every file of the API in `<api>` passes
//!   the schemas of `<published>`, opened for new fields and values, and its own.
//! - `zeabur-catalog check` prints the problems of the catalog's files and fails if there are any.
//! - `zeabur-catalog render [--out <dir>] [<id>…]` renders each release of each component on each
//!   profile and Kubernetes version into `out/render/` and checks the objects with kubeconform.
//! - `zeabur-catalog dry-run [--render <dir>]` starts a local API server and sends it every object
//!   that `zeabur-catalog render` wrote to `<dir>` (`out/render` by default) with
//!   `--dry-run=server`. It fails on what the API server refuses, and on an update it refuses from
//!   the pull request's base or between the profiles.
//! - `zeabur-catalog crosscheck [--out <file>]` writes `crosscheck.json` (by default), every
//!   template rendered with the values of its e2e run.
//! - `zeabur-catalog template render --api <dir> [--namespace <ns>] [--domain <host>]
//!   [--cluster-issuer <name>] [--values <file>] <id>` prints the objects of a template of the
//!   static API in `<dir>`, the `v1` directory of a release's `api-v1.tar.gz`. `--values` names a
//!   JSON object of input values.
//! - `zeabur-catalog template inputs --api <dir> [--namespace <ns>] <id>` prints a `List` of the
//!   Secrets that hold the template's generated inputs, with new random values.
//! - `zeabur-catalog digest <dir>…` prints the `sourceDigest` of each component or template
//!   directory.
//! - `zeabur-catalog schema` writes `schema/v1/` and `types/v1/catalog.ts` from the types of
//!   `zeabur-catalog-format`.
//! - `zeabur-catalog licenses-update` reads the SPDX License List's `licenses.json` from stdin and
//!   writes the OSI-approved identifiers to `config/licenses.yaml`.

use std::io::Read;
use std::path::Path;
use std::process::ExitCode;

mod api;
mod check;
mod compat;
mod crosscheck;
mod deploy;
mod dry_run;
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
        ["dry-run"] => dry_run("out/render"),
        ["dry-run", "--render", dir] => dry_run(dir),
        ["crosscheck"] => crosscheck::crosscheck(Path::new("."), Path::new("crosscheck.json")),
        ["crosscheck", "--out", out] => crosscheck::crosscheck(Path::new("."), Path::new(out)),
        ["template", action @ ("render" | "inputs"), rest @ ..] => template(action, rest),
        ["digest", dirs @ ..] if !dirs.is_empty() => digest(dirs),
        ["schema"] => write(),
        ["licenses-update"] => licenses_update(),
        _ => Err("usage: zeabur-catalog \
             api|check|compat|render|dry-run|crosscheck|template|digest|schema|licenses-update"
            .to_owned()),
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
    let dir = Path::new(out).join(format!("v{}", api::MAJOR));
    let root = Path::new(".");
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
    let problems = check::check(Path::new("."))?;
    for problem in &problems {
        output::error(&problem.to_string());
    }
    match problems.len() {
        0 => Ok(()),
        n => Err(format!("{n} problems")),
    }
}

fn compat(published: &str, api: &str) -> Result<(), String> {
    let problems = compat::compat(Path::new(published), Path::new(api))?;
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
    let problems = render::render(Path::new("."), &ids, Path::new(out))?;
    for problem in &problems {
        output::error(&problem.to_string());
    }
    match problems.len() {
        0 => Ok(()),
        n => Err(format!("{n} problems")),
    }
}

fn dry_run(render: &str) -> Result<(), String> {
    let problems = dry_run::dry_run(Path::new("."), Path::new(render))?;
    for problem in &problems {
        output::error(&problem.to_string());
    }
    match problems.len() {
        0 => Ok(()),
        n => Err(format!("{n} problems")),
    }
}

fn template(action: &str, args: &[&str]) -> Result<(), String> {
    const USAGE: &str = "usage: zeabur-catalog template render|inputs --api <dir> [--namespace \
                         <ns>] [--domain <host>] [--cluster-issuer <name>] [--values <file>] <id>";
    let mut api = None;
    let mut id = None;
    let mut options = deploy::Options {
        namespace: "default".to_owned(),
        ..deploy::Options::default()
    };
    let mut rest = args;
    while let Some((&arg, tail)) = rest.split_first() {
        let flag = match (arg, tail) {
            (
                "--api" | "--namespace" | "--domain" | "--cluster-issuer" | "--values",
                [value, ..],
            ) => Some((*value).to_owned()),
            (..) if !arg.starts_with("--") && id.is_none() => {
                id = Some(arg);
                None
            }
            _ => return Err(USAGE.to_owned()),
        };
        rest = if let Some(value) = flag {
            match arg {
                "--api" => api = Some(value),
                "--namespace" => options.namespace = value,
                "--domain" => options.domain = Some(value),
                "--cluster-issuer" => options.cluster_issuer = Some(value),
                _ => options.values = Some(value),
            }
            tail.get(1..).unwrap_or_default()
        } else {
            tail
        };
    }
    let (Some(api), Some(id)) = (api, id) else {
        return Err(USAGE.to_owned());
    };
    let dir = deploy::read(Path::new(&api), id)?;
    let text = if action == "render" {
        deploy::render(id, &dir, &options)?
    } else {
        let list = deploy::inputs(id, &dir, &options.namespace, deploy::os_random)?;
        serde_json::to_string_pretty(&list).map_err(|e| e.to_string())?
    };
    output::print(text.trim_end());
    Ok(())
}

fn digest(dirs: &[&str]) -> Result<(), String> {
    for dir in dirs {
        let digest = zeabur_catalog_engine::digest::source_digest(Path::new(dir))
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
    zeabur_catalog_format::schema::write_typescript(Path::new("types/v1"))
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
