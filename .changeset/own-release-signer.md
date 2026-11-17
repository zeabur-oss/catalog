---
default: minor
---

# Releases are signed by the catalog's own release workflow, and checksums.txt is gone

Verify `SHA256SUMS` with `--certificate-identity "https://github.com/zeabur-oss/catalog/.github/workflows/release.yml@refs/tags/<tag>"`, and attestations with `--signer-workflow zeabur-oss/catalog/.github/workflows/release.yml` ([releases](docs/reference/releases.mdx)). Releases up to v0.5.1 keep the identity of `zeabur-oss/.github`. `SHA256SUMS` already lists the two archives, so a release no longer has `checksums.txt`.
