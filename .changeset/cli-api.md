---
default: minor
---

# zeabur-catalog api writes the static API

`zeabur-catalog api --version <v> [--out <dir>]` writes `catalog.json`, `components/<id>.json`, `templates/<id>.json` and the schemas to `<dir>/v1/`, then checks every file against the schemas. The release builds the API with it instead of the Go harness. Its files hold the same data as before: the declarations and templates as written, the render of each profile at the newest Kubernetes version, the measurements, and requests, limits and storage as Kubernetes writes them.
