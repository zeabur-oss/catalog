---
default: patch
---

# Single-node memory requests follow what the default components use

Most default components ask for less memory on one node, such as Zeabur Builder's controller, metrics-server, CloudNativePG and the VictoriaMetrics operator's config reloaders. VictoriaMetrics, vmagent, vmalert, kube-state-metrics, cert-manager, the VictoriaMetrics operator and zot ask for more, because they used more than they asked for with 35 pods running.
