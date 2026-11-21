## Introduction

MOCO runs MySQL from a `MySQLCluster` object: one instance, or a semi-synchronous primary with replicas. It backs up to S3 and restores to a point in time. Install it when applications need MySQL itself. The operator is Apache-2.0. The MySQL images it runs are GPL-2.0.

MOCO needs cert-manager for the webhook and for the mTLS between the controller and its agents. The install dialog adds cert-manager first when the cluster has none. The controller manages Pods, StatefulSets, Services, Secrets, PVCs, CronJobs, Jobs and Roles for `MySQLCluster` objects in every namespace, and it copies certificates into them.

## Use it

In Zeabur OSS, open a project's **Add** menu and choose **MySQL**. Set the version and storage, then connect services and set up S3 backups in the database's **Backups** tab. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/).

## Remove it

Removing MOCO removes the controller and its webhooks. Zeabur OSS refuses while `MySQLCluster` objects exist, so delete the databases you no longer need first, and back up the rest. The CRDs stay, and so do the PVCs of deleted databases. Delete them by hand when you no longer need the data. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A database never becomes Healthy

Check the object with `kubectl -n <namespace> get mysqlcluster <name>` and its Pods with `kubectl -n <namespace> get pods`. Read the controller's logs with `kubectl -n moco-system logs deploy/moco-controller`.

### The API server rejects a `MySQLCluster` object with a webhook error

Check that the controller runs with `kubectl -n moco-system get pods`, and that its certificates are ready with `kubectl -n moco-system get certificate`.
