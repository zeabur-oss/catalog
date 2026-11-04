#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
source "$(dirname "$0")/lib.sh"

shellcheck scripts/*.sh scripts/ci/*.sh
actionlint
bun install --frozen-lockfile
bun check scripts/
