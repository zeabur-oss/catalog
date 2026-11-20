---
default: major
---

# Each component has its own directory

A component moved from `components/<id>.yaml` to `components/<id>/component.yaml`, with its READMEs (`README.md`, `README.zh-TW.md`, `README.zh-CN.md`) in the same directory, as templates are. A reader of the release bundle reads the new paths. `catalog validate` refuses any other file in `components/`.

A component's `sourceDigest` is now the digest of its directory without the READMEs, so every component has a new digest and is untested until the next e2e run. Editing only a README of a component or a template keeps its `sourceDigest`.
