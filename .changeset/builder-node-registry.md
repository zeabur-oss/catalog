---
default: patch
---

# Nodes pull Zeabur Builder's images without manual setup

On K3s, RKE2 and KubeSolo, the new `zot-node` DaemonSet points each node's container runtime at zot's fixed ClusterIP and gives it zot's CA, so a pod can pull `zot.zot.svc:5000/...` images right after you install the component. You no longer write `registries.yaml` by hand.
