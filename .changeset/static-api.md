---
default: major
---

# A static API replaces index.json

Each release publishes `api-v1.tar.gz`, and each release that isn't a pre-release publishes the same files at `https://oss-catalog.zeabur.com/v1/`. `v1/catalog.json` lists the release, the profiles, the Kubernetes versions, the pinned distributions and each component's `sourceDigest`. `v1/components/<id>.json` holds a component's declaration as written, what it renders for each profile it supports, with the replicas, requests, limits and storage of its workloads and claims, and its measurements. Each file passes its schema in `v1/schema/`, and the Go types of the catalog's tools generate those schemas.

`index.json` and `indexVersion` are gone: the release no longer has `index.json`, and `bundle.tar.gz` no longer carries it. The API holds facts only: `testedOn`, `installOrder` and the rounded `resources.work` are gone, and a reader works them out as the installer reference describes. A measurement keeps its `result`, including a failed run.

`schema/v1/component.schema.json` and `schema/v1/template.schema.json` are generated too. A template's localized `summary` must be a single line, like a component's, and a template input generates `random` values only.
