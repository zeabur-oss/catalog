---
default: minor
---

# API v1 has template measurements and READMEs

`v1/templates/<id>.json` has the template's `measurements`, as `v1/components/<id>.json` has the component's. Both have `readmes`, the locales of the READMEs that the API now publishes as Markdown files under `v1/components/<id>/` and `v1/templates/<id>/`. A reader no longer reads the repository for them.
