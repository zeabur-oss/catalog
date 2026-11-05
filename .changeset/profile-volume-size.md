---
default: major
---

# Switching profiles keeps zot and VictoriaMetrics running

zot's volume is 20 GiB and VictoriaMetrics' 5 GiB on both `single-node` and `multi-node`, the sizes `single-node` already had. Before, a switch changed the size: Kubernetes refused to update zot's StatefulSet, so its upgrade failed, and the VictoriaMetrics operator couldn't resize the `local-path` volume, so it stopped updating vmsingle and left it on the `single-node` memory limit. `local-path` ignores the size. On Longhorn, a new `multi-node` install starts smaller than before; grow the claim with `kubectl edit pvc` when it fills.

An install created on `multi-node` has a 50 GiB zot volume template and a 20 GiB VictoriaMetrics claim, so its first upgrade to this release needs three commands. Without them, zot's upgrade fails and the operator stops updating vmsingle. Run them, then `apply` again:

```sh
kubectl -n zot delete statefulset zot --cascade=orphan
kubectl -n observability annotate pvc vmsingle-vm-victoria-metrics-k8s-stack operator.victoriametrics.com/pvc-allow-volume-expansion=false
kubectl -n observability rollout restart deployment victoria-metrics-operator
```

The first keeps zot's Pod and volume, and the upgrade creates the StatefulSet again with the new template. The second tells the operator to leave the existing claim's size alone, and the third makes it try vmsingle again, which it does not do on its own until the VMSingle changes again.

Breaks: builds/zeabur-builder StatefulSet zot/zot
