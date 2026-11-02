---
default: minor
---

# The index publishes each component's memory requests, measured in L2

`measured.<env>.requestMemoryMiB` is the sum of the memory requests of the pods a component runs, read from the cluster after L2 settles. Jobs and finished pods are left out.
