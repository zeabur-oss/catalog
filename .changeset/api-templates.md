---
default: minor
---

# The API publishes the templates

`catalog api` writes `v1/templates/<id>.json` for each template: its `id` and `sourceDigest`, its `template` (`template.yaml` as JSON) and its `manifests` (the text of `manifests.yaml`). `v1/catalog.json` lists each template's `id` and `sourceDigest` in `templates`, and `v1/schema/` holds `template.schema.json` and `template-file.schema.json`.
