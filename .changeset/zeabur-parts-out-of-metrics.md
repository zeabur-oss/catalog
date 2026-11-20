---
default: patch
---

# VictoriaMetrics and VictoriaLogs carry no Zeabur rules

Zeabur's notification rules and its log access move to Zeabur, which applies them when it installs VictoriaMetrics and VictoriaLogs. They ship with the Zeabur version that uses them.

- VictoriaMetrics no longer installs the `zeabur-notifications` VMRule. Zeabur applies a VMRule of the same name, so a cluster that Zeabur manages needs no step. Without Zeabur, delete the old one: `kubectl -n observability delete vmrule zeabur-notifications`.
- The custom resource metrics are named `kube_customresource_*` instead of `zeabur_*`, such as `kube_customresource_cnpg_backup_created`.
- kube-state-metrics exports every label of Jobs and namespaces, not only Zeabur's.
- The VictoriaLogs NetworkPolicy no longer admits Zeabur Proxy. Zeabur adds its own policy for it.
