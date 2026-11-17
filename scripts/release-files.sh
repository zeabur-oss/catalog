#!/usr/bin/env bash
# The files of a release: bundle.tar.gz from `nix build .#release`, and api-v<N>.tar.gz, the static
# API of major N. `catalog api` renders charts that helm fetches, so the API is built outside the
# nix sandbox. Both are reproducible: the Release workflow verifies a draft by building them again.
#
#   scripts/release-files.sh <version> <dir>
set -euo pipefail
cd "$(dirname "$0")/.."

version=${1:?usage: scripts/release-files.sh <version> <dir>}
mkdir -p "${2:?usage: scripts/release-files.sh <version> <dir>}"
dir=$(cd "$2" && pwd)

go run ./tools/cmd/catalog validate
go run ./tools/cmd/catalog render
nix build .#release
install -m 644 result/bundle.tar.gz "$dir/bundle.tar.gz"

api=$(mktemp -d)
trap 'rm -r "$api"' EXIT
go run ./tools/cmd/catalog api -version "${version#v}" -out "$api"
major=$(ls "$api")
[[ "$major" =~ ^v[0-9]+$ ]] || { echo "catalog api wrote $major, not one v<N> directory" >&2; exit 1; }
tar -C "$api" --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner --mode=u+w,go-w,a+rX \
  -c "$major" | gzip -9n >"$dir/api-$major.tar.gz"
