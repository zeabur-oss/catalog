---
default: major
---

# Zeabur Builder installs chart 0.2.5 and its ClusterBuilders name their type

A ClusterBuilder now sets `spec.type` (`dockerfile`, `railpack` or `agent`) with the settings of that type in `spec.railpack` or `spec.agent`, in place of `spec.planner`, and `spec.frontend.type` is `dockerfile.v0` or `gateway.v0`. The two ClusterBuilders of the chart are already in the new shape. Before you upgrade, apply `crds/crds.yaml` of the chart, because Helm does not update CRDs, and rewrite each ClusterBuilder you made yourself. A Build's `spec.source` is also a union now, `type: git` with `git: {…}` or `type: archive` with `archive: {}`, so rewrite your Builds in that shape.

The upload Secrets of Zeabur Builder now hold the registry's CA in `ca.crt`, so an uploader that may read the Secret trusts zot without reading zot's own Secret. The controller may also read ServiceAccounts, which the optional agent builder needs.
