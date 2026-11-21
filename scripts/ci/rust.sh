#!/usr/bin/env bash
# The Rust workspace: format, lints, docs, tests, the generated schemas and the dependency policy.
set -euo pipefail
cd "$(dirname "$0")/../.."

cargo fmt --all --check
cargo lint
RUSTDOCFLAGS="-D warnings" cargo doc --workspace --no-deps --document-private-items --locked
cargo nextest run --workspace --all-features --locked --no-tests=warn
cargo run --locked -p zeabur-catalog -- schema
stale=$(git status --porcelain -- schema types)
if [[ -n "$stale" ]]; then
  echo "run cargo run -p zeabur-catalog -- schema and commit the result:" >&2
  echo "$stale" >&2
  exit 1
fi
cargo deny --locked check
