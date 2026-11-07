---
default: patch
---

# metrics-server answers from every node

On a `multi-node` cluster whose `kube-system` lets in only its own pods, as K3s's CIS hardening does, kube-apiserver could not reach the metrics-server replica on another node. Half of the `metrics.k8s.io` reads waited about a minute and then failed, so `kubectl top` and Zeabur's Nodes page hung. The component now has a NetworkPolicy that lets requests into metrics-server's port, as K3s's hardening guide does for its own metrics-server.
