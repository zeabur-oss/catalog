## Introduction

Velero backs up Kubernetes objects and volume files to S3-compatible storage, and restores them. Install it when you want to back up and restore whole projects. Database backups don't need it.

Velero is bound to `cluster-admin`. It reads and restores every object in the cluster, so only administrators should use it. By default no Pod is privileged. CSI snapshots are off. Velero copies volume files with Kopia, and only when you turn on the `fileBackup` input. Velero stores Secrets in a backup as they are, so the install generates two keys. One is the Kopia repository password. The other is a key that the S3 server uses to encrypt every backup object (SSE-C).

## Before you install

Prepare an S3 bucket outside this cluster. If you store backups in the cluster, losing the cluster also loses the backups. Then collect these inputs:

- `bucket`: the bucket name.
- `s3Url`: the S3 endpoint URL. Use HTTPS, and use storage that supports SSE-C.
- `region`: the bucket's region. The default is `us-east-1`.
- `credentials`: the bucket's keys as an AWS-style credentials file, with a `[default]` profile that sets `aws_access_key_id` and `aws_secret_access_key`.
- `fileBackup`: leave it `false` unless you need volume files in backups. When `true`, a node agent runs as root on every node, mounts host paths and holds Velero's token, and the `velero` namespace must allow the privileged Pod Security level.

## Use it

In Zeabur, open **Cluster** → **Backups** to set the backup location and schedule, run a backup, and restore. See [Back up a cluster and restore from a backup](https://oss.zeabur.com/docs/zeabur/cluster/backups/).

The two generated keys are also inside the backups they protect. Copy them out of the cluster now. Without them, no one can read your backups.

```sh
kubectl -n velero get secret velero-repo-credentials velero-backup-key -o yaml > velero-keys.yaml
```

## Remove it

Removing Velero deletes its Deployment and the node agent, if you turned it on. Backups and restores stop. The CRDs, and the Backup, Schedule and Restore objects, stay in the cluster. The backups in your bucket and the generated keys stay too. Keep the keys: you need them to read old backups. Before you remove Velero, let running backups finish. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### The backup location is not reachable

Check the location's phase and message with `kubectl -n velero get backupstoragelocations` and `kubectl -n velero describe backupstoragelocation default`. Then check the bucket, endpoint, region and keys. Read Velero's logs with `kubectl -n velero logs deploy/velero`.

### A backup ends as PartiallyFailed

When `fileBackup` is `false`, a backup that asks for volume files can't copy them, and it fails in part. Run `kubectl -n velero get backups`, then `velero backup describe <backup> --details` to see the failed items. Set `fileBackup` to `true` for the component, then run the backup again.
