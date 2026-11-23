---
default: patch
---

# zeabur-catalog compat checks the API against the published schemas

`zeabur-catalog compat <published> <api>` replaces the Go `catalog compat`. Every file of the new API must pass the schemas of the latest published release, which accept a field or a value they do not know, and its own schemas. The Go `catalog api` and `catalog compat` commands are removed.
