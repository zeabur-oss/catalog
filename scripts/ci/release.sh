#!/usr/bin/env bash
# What release.ts build makes. A go.sum change fails here first, with flake.nix's new vendorHash.
# shellcheck source-path=SCRIPTDIR
source "$(dirname "$0")/lib.sh"

nix build .#release
out="${CI_ARTIFACTS:-$root/out/ci}"
mkdir -p "$out"
cp result/* "$out/"
chmod u+w "$out"/*
(cd "$out" && sha256sum -c checksums.txt)
