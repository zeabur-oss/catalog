---
default: patch
---

# Each release is also at `https://oss-catalog.zeabur.com/v1/`

Each release that is not a pre-release copies its signed `index.json`, `bundle.tar.gz`, `SHA256SUMS` and `SHA256SUMS.sigstore.json` to `https://oss-catalog.zeabur.com/v1/`, so Zeabur can fetch new and fixed templates without a new Zeabur release.
