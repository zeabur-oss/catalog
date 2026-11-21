## Introduction

CloudNativePG runs PostgreSQL from a `Cluster` object, with replicas, failover and backups to S3. Install it when projects need PostgreSQL databases. This component installs two releases in the `cnpg-system` namespace: the operator and the Barman Cloud plugin that writes the backups.

The operator needs cert-manager. It secures the connection between the operator and the plugin with certificates. The install dialog adds cert-manager first when the cluster has none. The operator manages Pods, Services, Secrets and PVCs for `Cluster` objects in every namespace. The PodMonitor and the Grafana dashboard are off.

## Use it

In Zeabur OSS, open a project's **Add** menu and choose **PostgreSQL**. Set the version and storage, then connect services and set up S3 backups in the database's **Backups** tab. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/).

CloudNativePG writes backup files to the bucket in plaintext. Use a bucket with server-side encryption, and limit who can read it.

## Remove it

Removing the component removes the operator and the plugin. Zeabur OSS refuses while `Cluster` objects exist, so delete the databases you no longer need first, and back up the rest. The CRDs stay, and so do the PVCs of deleted databases. Delete them by hand when you no longer need the data. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A database never becomes Ready

Check the object and its events with `kubectl -n <namespace> describe cluster <name>`, and list its Pods with `kubectl -n <namespace> get pods`. Read the operator's logs with `kubectl -n cnpg-system logs deploy/cnpg-cloudnative-pg`.

### Backups don't start

Check that the plugin runs with `kubectl -n cnpg-system get pods`, and that its certificates are ready with `kubectl -n cnpg-system get certificate`. The plugin needs `barman-cloud-client` and `barman-cloud-server`.
