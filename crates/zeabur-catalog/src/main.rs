//! Zeabur Catalog's command. Run it from the repository root; `zeabur-catalog help` lists the
//! subcommands.

use std::path::{Path, PathBuf};
use std::process::ExitCode;

use clap::{Parser, Subcommand};

mod api;
mod check;
mod compat;
mod crosscheck;
mod dry_run;
mod lint;
mod policy;
mod render;
mod secrets;
mod templates;
mod values;

#[derive(Parser)]
#[command(
    name = "zeabur-catalog",
    about = "Checks, renders and publishes the Zeabur Catalog"
)]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand)]
enum Command {
    /// Writes the static API of the release to `<out>/v1/` and checks it against its schemas.
    Api {
        /// The release version.
        #[arg(long)]
        version: String,
        #[arg(long, default_value = "out/api")]
        out: PathBuf,
    },
    /// Prints the problems of the catalog's files and fails if there are any.
    Check,
    /// Fails unless every file of the API in `<api>` passes the schemas of `<published>`, opened
    /// for new fields and values, and its own.
    Compat { published: PathBuf, api: PathBuf },
    /// Renders each release of each component on each profile and Kubernetes version, and each
    /// template, and checks the objects.
    Render {
        #[arg(long, default_value = "out/render")]
        out: PathBuf,
        /// Only these components and templates.
        ids: Vec<String>,
    },
    /// Starts a local API server and sends it every object that `render` wrote with
    /// `--dry-run=server`. It fails on what the API server refuses, and on an update it refuses
    /// from the pull request's base or between the profiles.
    DryRun {
        #[arg(long, default_value = "out/render")]
        render: PathBuf,
    },
    /// Writes every template rendered with the values of its e2e run.
    Crosscheck {
        #[arg(long, default_value = "crosscheck.json")]
        out: PathBuf,
    },
    /// Prints the `sourceDigest` of each component or template directory.
    Digest {
        #[arg(required = true)]
        dirs: Vec<PathBuf>,
    },
    /// Writes `schema/v1/` from the types of `zeabur-catalog-format`.
    Schema,
}

fn main() -> ExitCode {
    let root = Path::new(".");
    let result = match Cli::parse().command {
        Command::Api { version, out } => api(&version, &out),
        Command::Check => check::check(root).and_then(|p| report(&p)),
        Command::Compat { published, api } => {
            compat::compat(&published, &api).and_then(|p| report(&p))
        }
        Command::Render { out, ids } => render::render(root, &ids, &out).and_then(|p| report(&p)),
        Command::DryRun { render } => dry_run::dry_run(root, &render).and_then(|p| report(&p)),
        Command::Crosscheck { out } => crosscheck::crosscheck(root, &out),
        Command::Digest { dirs } => digest(&dirs),
        Command::Schema => schema(),
    };
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            output::error(&e);
            ExitCode::FAILURE
        }
    }
}

/// Prints each problem, and fails if there is one.
fn report<P: std::fmt::Display>(problems: &[P]) -> Result<(), String> {
    for problem in problems {
        output::error(&problem.to_string());
    }
    match problems.len() {
        0 => Ok(()),
        n => Err(format!("{n} problems")),
    }
}

fn api(version: &str, out: &Path) -> Result<(), String> {
    let dir = out.join(format!("v{}", api::MAJOR));
    let root = Path::new(".");
    api::write(root, version, &api::commit(root)?, &dir)?;
    report(&api::check(
        &dir,
        &api::Schemas::read(&dir.join("schema"))?,
    )?)
}

fn digest(dirs: &[PathBuf]) -> Result<(), String> {
    for dir in dirs {
        let digest = zeabur_catalog_engine::digest::source_digest(dir)
            .map_err(|e| format!("{}: {e}", dir.display()))?;
        output::print(&digest);
    }
    Ok(())
}

fn schema() -> Result<(), String> {
    for (name, text) in zeabur_catalog_format::schema::schemas() {
        let path = format!("schema/v1/{name}");
        std::fs::write(&path, text).map_err(|e| format!("{path}: {e}"))?;
    }
    Ok(())
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
