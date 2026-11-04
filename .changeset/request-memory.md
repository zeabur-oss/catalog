---
default: minor
---

# The index publishes each component's memory requests, measured in the full tier

`measured.<env>.requests.memoryMiB` is the sum of the memory requests of the pods a component runs, read from the cluster after the full tier settles. Jobs and finished pods are left out.
