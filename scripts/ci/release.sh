#!/usr/bin/env bash
# What a release builds, at flake.nix's version. A go.sum change fails here first, with flake.nix's
# new vendorHash.
set -euo pipefail
cd "$(dirname "$0")/../.."

out="${CI_ARTIFACTS:-$PWD/out/ci}"
rm -rf "$out"
scripts/release-files.sh "v$(nix eval --raw .#catalog.version)" "$out"
