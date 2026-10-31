#!/usr/bin/env bash
# One L0 job, as .github/workflows/ci.yml runs it (through zeabur-oss/.github) and as anyone runs it
# locally: the workflow only checks out and installs Nix, then calls this script, which enters the
# Nix dev shell itself. L1 and L2 stay in ci.yml and e2e.yml (docs/contribute/tests.mdx).
#
#   scripts/ci.sh <job>      # run one job
#   scripts/ci.sh --list     # the job names
#
# A job leaves what CI uploads in $CI_ARTIFACTS: the release job, the output of nix build .#release.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

jobs=(lint test l0 release)

if [[ "${1:-}" == --list ]]; then
  printf '%s\n' "${jobs[@]}"
  exit 0
fi
job="${1:?usage: scripts/ci.sh <job> | --list}"
printf '%s\n' "${jobs[@]}" | grep -qx -- "$job" ||
  { echo "error: no job named $job (scripts/ci.sh --list)" >&2; exit 1; }

# The marker tells a re-run inside the dev shell apart from a call outside it.
if [[ -z "${CATALOG_CI_SHELL:-}" ]]; then
  export CATALOG_CI_SHELL=1
  exec nix develop "$root" --command "$root/scripts/ci.sh" "$@"
fi

case "$job" in
  lint)
    shellcheck scripts/*.sh
    actionlint
    bun install --frozen-lockfile
    bun fmt --check scripts/release.ts
    bun check scripts/
    ;;
  test) go test ./... ;;
  # Validate, render, template lint and a server-side dry run on a local API server.
  l0) go run ./tools/cmd/catalog e2e -tier l0 ;;
  # What release.ts build makes. A go.sum change fails here first, with flake.nix's new vendorHash.
  release)
    nix build .#release
    out="${CI_ARTIFACTS:-$root/out/ci}"
    mkdir -p "$out"
    cp result/* "$out/"
    chmod u+w "$out"/*
    (cd "$out" && sha256sum -c checksums.txt)
    ;;
esac
