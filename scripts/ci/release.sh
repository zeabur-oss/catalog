#!/usr/bin/env bash
# What release.ts build makes. A go.sum change fails here first, with flake.nix's new vendorHash.
set -euo pipefail
cd "$(dirname "$0")/../.."

nix build .#release
out="${CI_ARTIFACTS:-$PWD/out/ci}"
mkdir -p "$out"
cp result/* "$out/"
chmod u+w "$out"/*
(cd "$out" && sha256sum -c checksums.txt)
