---
default: patch
---

# Zeabur Builder installs chart 0.2.5

The upload Secrets of Zeabur Builder now hold the registry's CA in `ca.crt`, so an uploader that may read the Secret trusts zot without reading zot's own Secret. The controller may also read ServiceAccounts, which the optional agent builder needs.

The chart's ClusterBuilders name their type, with `spec.railpack` in place of `spec.planner`. A ClusterBuilder you made yourself needs the same change, and a Build's `spec.source` is now `type: git` with `git: {…}` or `type: archive` with `archive: {}`. Apply `crds/crds.yaml` of the chart before you upgrade, because Helm does not update CRDs.
