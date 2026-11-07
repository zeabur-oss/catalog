---
default: patch
---

# Zeabur Builder moves to chart 0.2.1

buildkitd now always runs in a user namespace, and a build starts again after the buildkitd cache size or storage class changes. The component's `kubeVersion` (1.34 or newer) already covers the user namespaces this needs.
