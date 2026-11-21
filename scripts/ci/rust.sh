#!/usr/bin/env bash
# The Rust workspace: format, lints, docs, tests and the dependency policy.
set -euo pipefail
cd "$(dirname "$0")/../.."

cargo fmt --all --check
cargo lint
RUSTDOCFLAGS="-D warnings" cargo doc --workspace --no-deps --document-private-items --locked
cargo nextest run --workspace --all-features --locked --no-tests=warn
cargo deny --locked check
