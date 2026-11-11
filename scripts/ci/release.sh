#!/usr/bin/env bash
# What a release builds: release.ts build, at flake.nix's version. A go.sum change fails here first,
# with flake.nix's new vendorHash.
set -euo pipefail
cd "$(dirname "$0")/../.."

bun install --frozen-lockfile
out="${CI_ARTIFACTS:-$PWD/out/ci}"
rm -rf "$out"
bun scripts/release.ts build "v$(nix eval --raw .#catalog.version)" linux "$(uname -m)" "$out"
(cd "$out" && sha256sum -c checksums.txt)
