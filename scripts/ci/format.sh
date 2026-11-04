#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
source "$(dirname "$0")/lib.sh"

unformatted=$(gofmt -l .)
if [[ -n "$unformatted" ]]; then
  echo "gofmt -w these files:" >&2
  echo "$unformatted" >&2
  exit 1
fi
bun install --frozen-lockfile
bun fmt --check scripts/release.ts
