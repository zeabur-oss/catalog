---
default: patch
---

# Switching profiles keeps zot and VictoriaMetrics running

zot's volume is 20 GiB and VictoriaMetrics' 5 GiB on both `single-node` and `multi-node`, the sizes `single-node` already had. Before, a switch changed the size: Kubernetes refused to update zot's StatefulSet, so its upgrade failed, and the VictoriaMetrics operator couldn't resize the `local-path` volume, so it stopped updating vmsingle and left it on the `single-node` memory limit. `local-path` ignores the size. On Longhorn, a new `multi-node` install starts smaller than before; grow the claim with `kubectl edit pvc` when it fills.
