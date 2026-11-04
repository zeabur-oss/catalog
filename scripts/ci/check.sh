#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."

go run ./tools/cmd/catalog e2e -tier check
