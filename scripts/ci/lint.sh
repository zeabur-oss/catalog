#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."

unformatted=$(gofmt -l .)
if [[ -n "$unformatted" ]]; then
  echo "gofmt -w these files:" >&2
  echo "$unformatted" >&2
  exit 1
fi
shellcheck scripts/ci/*.sh
scripts/ci/patterns.sh
jactionlint
./scripts/ci/patterns.sh
