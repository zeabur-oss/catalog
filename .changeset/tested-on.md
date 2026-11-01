---
default: minor
---

# `index.json` gives each component `testedOn`, the distributions among k3s, RKE2 and KubeSolo where its e2e test passed at the pinned version

A failed run marks its environment `failed` in `measurements/`. The distribution then stays out of `testedOn` until a run passes again, and `measured` in the index leaves the environment out.
