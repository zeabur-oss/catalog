## Introduction

Dragonfly Operator runs Dragonfly, a Redis-compatible in-memory store, from a `Dragonfly` object: a primary with replicas, failover, and snapshots to a volume. Install it when projects need a Redis-compatible store.

The operator is Apache-2.0. The Dragonfly images it runs are BUSL-1.1, which is source-available and not an OSI-approved open-source licence. Read its restrictions before you use it, especially if you offer hosted database services to others.

The operator manages Pods, StatefulSets, Services and PodDisruptionBudgets for `Dragonfly` objects in every namespace. A `kube-rbac-proxy` container guards its metrics port.

## Use it

In Zeabur, open a project's **Add** menu and choose **Dragonfly**. Set the storage, then connect services. Dragonfly has no **Backups** tab. It keeps snapshots on its volume and loads the newest one when it starts. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/).

## Remove it

Removing the operator leaves the CRD in place, and with it every `Dragonfly` object. Nothing manages those objects until the operator is back. Zeabur refuses to remove the component while `Dragonfly` objects exist. Delete the databases you no longer need first. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A database never becomes Ready

Check the object with `kubectl -n <namespace> get dragonfly <name>` and its Pods with `kubectl -n <namespace> get pods`. Read the operator's logs with `kubectl -n dragonfly-operator-system logs deploy/dragonfly-operator -c manager`.
