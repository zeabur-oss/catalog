#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."

cargo run --locked -p zeabur-catalog -- check
cargo run --locked -p zeabur-catalog -- render --out out/render
cargo run --locked -p zeabur-catalog -- dry-run --render out/render
