---
default: patch
---

# `zeabur-catalog-engine` has the install's object types and progress

The engine crate now holds the labels, annotations and field manager an install writes, the K3s/RKE2 `HelmChart` and `HelmChartConfig` types, and the progress of a chart, an object or a health check, moved from Zeabur with their tests.
