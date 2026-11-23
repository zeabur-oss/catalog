#!/usr/bin/env bash
# The files of a release: bundle.tar.gz from `nix build .#release`, and api-v<N>.tar.gz, the static
# API of major N. `catalog api` renders charts that helm fetches, so the API is built outside the
# nix sandbox. Both are reproducible: the Release workflow verifies a draft by building them again.
# CI builds them at flake.nix's version, so a go.sum change fails here first, with flake.nix's new
# vendorHash. A change inside v<N> keeps every file passing the schemas of the latest published
# release's api-v<N>.tar.gz; a breaking change goes to v<N+1>, which has none yet.
#
#   scripts/ci/release.sh [<version> <dir>]
set -euo pipefail
cd "$(dirname "$0")/../.."

version=${1:-v$(nix eval --raw .#catalog.version)}
if [[ $# == 0 ]]; then
  out=${CI_ARTIFACTS:-$PWD/out/ci}
  rm -rf "$out"
else
  out=${2:?usage: scripts/ci/release.sh [<version> <dir>]}
fi
mkdir -p "$out"
dir=$(cd "$out" && pwd)

cargo run --locked -p zeabur-catalog -- check
cargo run --locked -p zeabur-catalog -- render
nix build .#release
install -m 644 result/bundle.tar.gz "$dir/bundle.tar.gz"

api=$(mktemp -d)
published=$(mktemp -d)
trap 'rm -r "$api" "$published"' EXIT
go run ./tools/cmd/catalog api -version "${version#v}" -out "$api"
major=$(ls "$api")
[[ "$major" =~ ^v[0-9]+$ ]] || { echo "catalog api wrote $major, not one v<N> directory" >&2; exit 1; }
tar -C "$api" --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner --mode=u+w,go-w,a+rX \
  -c "$major" | gzip -9n >"$dir/api-$major.tar.gz"

status=$(curl -sSL -o "$published/api.tar.gz" -w '%{http_code}' \
  "https://github.com/zeabur-oss/catalog/releases/latest/download/api-$major.tar.gz")
case $status in
  200)
    tar -xzf "$published/api.tar.gz" -C "$published"
    go run ./tools/cmd/catalog compat "$published/$major" "$api/$major"
    ;;
  404) echo "no published release has api-$major.tar.gz yet; nothing to compare" ;;
  *) echo "downloading the latest release's api-$major.tar.gz: HTTP $status" >&2; exit 1 ;;
esac
