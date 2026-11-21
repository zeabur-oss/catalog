## Introduction

K8up backs up the contents of PersistentVolumeClaims to S3-compatible storage with restic. Install it when services need backups of their own volumes. It backs up one service's files, not the whole cluster. Database backups don't need it.

K8up backs up only PVCs that carry the annotation `k8up.io/backup=true`, so installing it backs up nothing by itself. Its controller watches K8up objects and reads PVCs, PersistentVolumes and Pods in every namespace. It creates backup Jobs, ServiceAccounts and RoleBindings in the namespace of the volume. Each backup Job mounts the volume in its own namespace only.

## Use it

In Zeabur OSS, open a service's **Settings** → **Storage** and click **Back up this volume**. Have an S3 bucket and keys ready, or an S3 destination that an administrator registered. Zeabur OSS creates the K8up Schedule and the Secret that holds the restic password. See [Set up volume backups](https://oss.zeabur.com/docs/zeabur/configure/volumes/#set-up-volume-backups).

Copy the `RESTIC_PASSWORD` from the backup Secret to a safe place. Without it, you can't read the backups in S3.

## Remove it

Removing K8up stops all scheduled backups. When the chart goes, a cleanup Job deletes the `pod-executor` Role and RoleBinding that K8up added in each namespace. The CRDs stay, and so do the Schedule objects, the backup Secrets and the data in your S3 buckets. Schedules do nothing until K8up is back. To stop one for good, delete it with `kubectl -n <namespace> delete schedule.k8up.io <name>`. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A backup never starts or fails

Check that the PVC carries `k8up.io/backup=true`: `kubectl -n <namespace> get pvc <name> -o jsonpath='{.metadata.annotations}'`. Then list the runs with `kubectl -n <namespace> get backups.k8up.io`, and read the operator's logs with `kubectl -n k8up-system logs deploy/k8up`.
