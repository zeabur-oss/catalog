---
default: minor
---

# zeabur-catalog digest prints the source digest

`zeabur-catalog digest <dir>…` prints the `sourceDigest` of each component or template directory, from the new `digest` module of `zeabur-catalog-engine`. The digests are the same as before. The Go harness calls it, so the API files and the measurements share one definition.
