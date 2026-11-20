## Introduction

System Upgrade Controller upgrades K3s or RKE2 on the nodes from Plans. A Plan names the nodes and the target version. The controller then upgrades one node at a time (or as many as `concurrency` allows), with cordon and drain, by running a privileged Job on each node.

The component is optional. Install it for Plans that do not change the Kubernetes version, such as OS package upgrades, or when nothing else upgrades your nodes. Do not install it when a node tool pins the K3s or RKE2 version, such as the Zeabur installer. That tool puts the pinned version back on a node that a Plan upgraded. The component works on K3s and RKE2 only, because its Plans replace the K3s or RKE2 binary.

The controller cordons and drains nodes. Its Jobs are privileged and run Plan images on the host.

## Use it

Zeabur has no screen for it. Create a Plan in the `system-upgrade` namespace. This Plan upgrades the server nodes of a K3s cluster to a version you choose:

```yaml
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: server-plan
  namespace: system-upgrade
spec:
  concurrency: 1
  cordon: true
  nodeSelector:
    matchExpressions:
      - key: node-role.kubernetes.io/control-plane
        operator: In
        values: ["true"]
  serviceAccountName: system-upgrade
  upgrade:
    image: rancher/k3s-upgrade
  version: <K3s release, such as v1.33.4+k3s1>
```

Upgrade server nodes before agent nodes, and do not skip a minor version. For an agent Plan and RKE2 (`rancher/rke2-upgrade`), see [K3s automated upgrades](https://docs.k3s.io/upgrades/automated) and [RKE2 automated upgrades](https://docs.rke2.io/upgrades/automated_upgrade). Follow progress with `kubectl -n system-upgrade get plans,jobs`.

## Remove it

Removing the controller stops all Plans. Nodes that a Plan already upgraded keep their version. Check first that no upgrade Job is running: `kubectl -n system-upgrade get jobs`. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A Plan does nothing

Create the Plan in the `system-upgrade` namespace, the namespace of the controller. Run `kubectl -n system-upgrade get plans -o wide` and read the controller's logs with `kubectl -n system-upgrade logs deploy/system-upgrade-controller`. Check that the `nodeSelector` matches your nodes with `kubectl get nodes --show-labels`.
