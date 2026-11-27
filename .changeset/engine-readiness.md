---
default: patch
---

# `zeabur-catalog-engine` has kstatus readiness and the Pod Security levels

The engine crate now holds the kstatus readiness rules, the Pod Security level with its namespace labels, and the check for Kubernetes' own namespaces, moved from Zeabur with their tests.
