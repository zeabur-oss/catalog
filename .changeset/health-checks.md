---
default: minor
---

# A component's `health` lists read-only checks that an installer can run on a cluster

Each is one `kubectl wait`, with a timeout and advice for when it fails. e2e runs them after install. Longhorn, cert-manager, Traefik and metrics-server have them.
