#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."

cargo run --locked -p zeabur-catalog -- check
cargo run --locked -p zeabur-catalog -- render
go run ./tools/cmd/catalog e2e -tier check
