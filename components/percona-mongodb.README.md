## Introduction

Percona Operator for MongoDB runs MongoDB replica sets from a `PerconaServerMongoDB` object, with scheduled backups to S3 through Percona Backup for MongoDB. Install it when projects need MongoDB.

The operator is Apache-2.0. The Percona Server for MongoDB images it runs are under the SSPL-1.0, which is not an OSI-approved open-source licence. Read the SSPL before you use it.

The operator manages Pods, Services, Secrets and PVCs for MongoDB clusters in every namespace. The component turns off the operator's telemetry, which reports to Percona by default.

## Use it

In Zeabur, open a project's **Add** menu and choose **MongoDB**. Set the version and storage, then connect services and set up S3 backups in the database's **Backups** tab. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/).

## Remove it

Removing the operator leaves the CRDs in place. Zeabur refuses while `PerconaServerMongoDB` objects exist, so delete the databases you no longer need first, and back up the rest. The PVCs of deleted databases stay. Delete them by hand when you no longer need the data. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A database never becomes ready

Check the object with `kubectl -n <namespace> get psmdb <name>` and its Pods with `kubectl -n <namespace> get pods`. Read the operator's logs with `kubectl -n psmdb-operator logs deploy/psmdb-operator`.

### A MongoDB 8 Pod crashes when it starts

MongoDB 8 does not start on Linux 6.19 and newer (SERVER-121912). Check the node's kernel with `uname -r`. Zeabur adds `GLIBC_TUNABLES=glibc.pthread.rseq=1` to the MongoDB 8 databases it creates. For a database you write yourself, use a 7.0 image.
