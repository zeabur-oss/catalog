---
default: patch
---

# VictoriaMetrics and VictoriaLogs carry no Zeabur rules

Zeabur's notification rules and its log access move to the Zeabur chart, which ships them with the Zeabur version that uses them.

- VictoriaMetrics no longer installs the `zeabur-notifications` VMRule. Upgrading the component leaves the old one in place, so delete it once Zeabur installs its own: `kubectl -n observability delete vmrule zeabur-notifications`.
- The custom resource metrics are named `kube_customresource_*` instead of `zeabur_*`, such as `kube_customresource_cnpg_backup_created`.
- kube-state-metrics exports every label of Jobs and namespaces, not only Zeabur's.
- The VictoriaLogs NetworkPolicy no longer admits Zeabur Proxy. The Zeabur chart adds its own policy for it.
