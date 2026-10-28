#!/usr/bin/env bash
# The stages of a release, as zeabur-oss/.github's release.yml calls them through
# .github/workflows/release.yml. Each runs in the Nix dev shell of the checkout it is called from.
#
#   scripts/release.sh check   <version>                                the version is flake.nix's and
#                                                                       has a CHANGELOG.md section
#   scripts/release.sh build   <version> <os> <arch> <dist>             validate, nix build .#release
#   scripts/release.sh publish <version> <artifacts> <published>        nothing to push: []
#   scripts/release.sh assets  <version> <artifacts> <release>          sign, SHA256SUMS, notes
#   scripts/release.sh verify  <version> <os> <arch> <downloaded>       checksums, signatures,
#                                                                       attestations, rebuild
#
# The release is data, the same on every platform: one platform builds it.
set -euo pipefail

stage="${1:?usage: scripts/release.sh check|build|publish|assets|verify <version> ...}"

# verify-only runs a newer release.sh from verification/scripts in the tag's checkout: the dev
# shell is the one of the working directory, which is the commit being released.
if [[ -z "${CATALOG_RELEASE_SHELL:-}" ]]; then
  export CATALOG_RELEASE_SHELL=1
  exec nix develop "$PWD" --command "$0" "$@"
fi

version="${2:?version}"
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "error: $version is not vX.Y.Z" >&2; exit 1; }

# The files nix build .#release makes; each gets a Sigstore bundle <file>.sigstore.json.
files=(bundle.tar.gz index.json checksums.txt)

notes() {
  awk -v v="${version#v}" '$0 ~ "^## \\[" v "\\]" { on = 1; next } on && /^## / { exit } on' CHANGELOG.md
}

case "$stage" in
  check)
    flake_version=$(nix eval --raw .#catalog.version)
    [[ "$flake_version" == "${version#v}" ]] ||
      { echo "error: flake.nix has version $flake_version, not ${version#v}" >&2; exit 1; }
    [[ -n "$(notes | tr -d '[:space:]')" ]] ||
      { echo "error: CHANGELOG.md has no section ## [${version#v}]" >&2; exit 1; }
    ;;

  build)
    dist="${5:?dist}"
    go run ./tools/cmd/catalog validate
    go run ./tools/cmd/catalog render
    nix build .#release
    mkdir -p "$dist"
    cp result/* "$dist/"
    chmod u+w "$dist"/*
    # Smoke: the checksums hold, the index is JSON and the bundle carries it.
    (cd "$dist" && sha256sum -c checksums.txt)
    jq -e .indexVersion "$dist/index.json" >/dev/null
    tar -xzOf "$dist/bundle.tar.gz" index.json | cmp - "$dist/index.json"
    ;;

  # The catalog is files on the release; it pushes no image or chart.
  publish)
    published="${4:?published}"
    mkdir -p "$published"
    echo '[]' >"$published/subjects.json"
    ;;

  assets)
    artifacts="${3:?artifacts}" out="${4:?release}"
    shopt -s nullglob
    candidates=("$artifacts"/candidate-*)
    (( ${#candidates[@]} == 1 )) ||
      { echo "error: expected one candidate in $artifacts, found ${#candidates[@]}" >&2; exit 1; }
    mkdir -p "$out/assets"
    for f in "${files[@]}"; do
      cp "${candidates[0]}/$f" "$out/assets/$f"
      # Keyless, with this job's GitHub OIDC token. docs/reference/releases.mdx verifies it.
      cosign sign-blob --yes --bundle "$out/assets/$f.sigstore.json" "$out/assets/$f"
    done
    (cd "$out/assets" && sha256sum -- * >SHA256SUMS)
    notes >"$out/notes.md"
    ;;

  verify)
    downloaded="${5:?downloaded}"
    : "${RELEASE_CERT_IDENTITY_REGEXP:?}" "${RELEASE_SIGNER_WORKFLOW:?}"
    : "${RELEASE_SOURCE_REPOSITORY:?}" "${RELEASE_SOURCE_REF:?}"
    (cd "$downloaded" && sha256sum -c SHA256SUMS && sha256sum -c checksums.txt)
    for f in "${files[@]}"; do
      cosign verify-blob "$downloaded/$f" --bundle "$downloaded/$f.sigstore.json" \
        --certificate-oidc-issuer https://token.actions.githubusercontent.com \
        --certificate-identity-regexp "$RELEASE_CERT_IDENTITY_REGEXP" \
        --certificate-github-workflow-repository "$RELEASE_SOURCE_REPOSITORY" \
        --certificate-github-workflow-ref "$RELEASE_SOURCE_REF"
    done
    awk '{ print $2 }' "$downloaded/SHA256SUMS" | while read -r f; do
      gh attestation verify "$downloaded/$f" --repo "$RELEASE_SOURCE_REPOSITORY" \
        --signer-workflow "$RELEASE_SIGNER_WORKFLOW" >/dev/null
      echo "$f: attestation OK"
    done
    # The release is what nix build .#release makes at the tag, byte for byte.
    nix build .#release
    for f in "${files[@]}"; do
      cmp "result/$f" "$downloaded/$f"
    done
    ;;

  *)
    echo "error: no stage $stage" >&2
    exit 1
    ;;
esac
