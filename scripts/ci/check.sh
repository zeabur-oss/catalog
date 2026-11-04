#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
source "$(dirname "$0")/lib.sh"

go run ./tools/cmd/catalog e2e -tier check
