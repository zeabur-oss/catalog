---
default: patch
---

# A changed component is no longer listed in testedOn until a run of the new file passes

Each measured environment records `sourceDigest`, a digest of the component file, or of `template.yaml` and `manifests.yaml` for a template. `testedOn` counts only passing runs whose digest equals the current file's. Runs without the field no longer count, so every component reads as untested until the next weekly run measures it again.
