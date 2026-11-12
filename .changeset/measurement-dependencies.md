---
default: minor
---

# Measurements record the digests of the components a run installed first

Each measurement in `v1/components/<id>.json` has `dependencies`, the `sourceDigest` of each component that the run installed before this one, by ID. Measurements whose inputs differ from the release's files and pinned Kubernetes and distribution releases are no longer published: a run of older files, of another `kubernetesVersion` on kind, or of another `distributionVersion` on a host distribution.
