---
default: patch
---

# VictoriaMetrics on `multi-node` requests as much memory as on `single-node`

`multi-node` now keeps the same metrics as `single-node`: node-exporter runs only the collectors the rules query, kube-state-metrics watches only the objects that map pods to workloads, and the API server's metrics and the kubelet's histograms are no longer collected. vmsingle requests 224 MiB instead of 512 MiB with a 1 GiB limit, and vmagent requests 128 MiB with a 512 MiB limit. vmalert and Alertmanager get the `single-node` requests too. The alert rules that read the API server's metrics go with them, as they already did on `single-node`. `multi-node` still keeps two weeks of history.
