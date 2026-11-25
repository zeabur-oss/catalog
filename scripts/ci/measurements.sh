#!/usr/bin/env bash
# A release pull request fails while a measurement is stale: what weekly.yml would test, as
# `catalog list -tier full` chooses it. Merge the ci/measurements pull request, then run CI again.
set -euo pipefail
cd "$(dirname "$0")/../.."

stale=$(go run ./tools/cmd/catalog list -tier full | jq -r '.include[] | "\(.profile) \(.arch) \(.component // .template)"')
if [[ -n "$stale" ]]; then
  echo "These measurements are stale (profile, architecture, ID):" >&2
  echo "$stale" >&2
  echo "Merge the ci/measurements pull request, or run weekly.yml for them, then run CI again." >&2
  exit 1
fi
