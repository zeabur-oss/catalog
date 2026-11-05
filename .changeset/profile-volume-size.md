---
default: patch
---

# Switching profiles keeps zot and VictoriaMetrics running

zot's volume is 50 GiB and VictoriaMetrics' 20 GiB on both `single-node` and `multi-node`. Before, a switch changed the size: Kubernetes refused to update zot's StatefulSet, so its upgrade failed, and the VictoriaMetrics operator couldn't resize the `local-path` volume, so it stopped updating vmsingle and left it on the `single-node` memory limit. `local-path` ignores the size and Longhorn only takes the space that's written, so the larger size costs nothing.
