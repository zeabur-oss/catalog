---
default: major
---

# Components no longer have `replaces`

`builds/zeabur-builder` no longer lists `container-registry/zot`, `index.json` and `schema/v1/component.schema.json` no longer have `replaces`, and `conformance/select/` no longer has the `replaced` error. An installer reports `container-registry/zot` in `enable` or `disable` as unknown. An installer no longer knows that Zeabur Builder's zot is the old component, so on a server that still has `container-registry/zot`, remove it before you install Builder. With zeabur-installer, run `kubectl -n kube-system delete helmchart zot`, wait until `kubectl -n zot get pods` lists none, then `kubectl delete namespace zot`. Builds push their images again.
