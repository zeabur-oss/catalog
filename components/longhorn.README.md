## Introduction

Longhorn is block storage that keeps copies of each volume on other nodes. A volume survives the loss of its node, follows its Pod to another node, and can grow in place. Install it on a multi-node cluster, where it becomes the default storage class. On a single node it is optional: a copy has no other node to go to, but you still get expandable volumes and snapshots. It does not run on KubeSolo.

The component asks for up to three copies per volume, one per node, and uses as many as you have nodes. With fewer nodes than copies, volumes still attach and gain the missing copies as nodes join. On a single node, `local-path` stays the default storage class and the `longhorn` class asks for one copy.

Longhorn runs privileged Pods on every node. They mount `/dev`, `/proc`, `/boot`, `/etc` and `/var/lib/longhorn` from the host, and they manage Pods, PersistentVolumes and storage classes in the whole cluster. The Longhorn UI has no login. It has no Ingress, and a NetworkPolicy blocks other Pods from reaching it.

## Before you install

Prepare every node first:

- open-iscsi, with `iscsid` running and the `iscsi_tcp` module loaded.
- An NFSv4 client, for ReadWriteMany volumes and NFS backups.
- `bash`, `curl`, `findmnt`, `grep`, `awk`, `blkid` and `lsblk`.
- If `multipathd` runs, a blacklist entry for Longhorn's disks. See [the Longhorn multipath guide](https://longhorn.io/kb/troubleshooting-volume-with-multipath/).

Run `longhornctl check preflight` on a node to check it. After the install, `kubectl -n longhorn-system get nodes.longhorn.io` shows the same result in each node's conditions.

## Use it

In Zeabur, pick **Replicated disk** under **Storage class** when you add a volume. See [Choose a storage class](https://oss.zeabur.com/docs/zeabur/configure/volumes/#choose-a-storage-class). Outside Zeabur, set `storageClassName: longhorn` on the PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata: {name: data, namespace: <namespace>}
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: longhorn
  resources:
    requests: {storage: 1Gi}
```

To open the Longhorn UI, run `kubectl -n longhorn-system port-forward svc/longhorn-frontend 8080:80`, then browse to `http://localhost:8080`. See the [Longhorn documentation](https://longhorn.io/docs/1.13.0/).

## Remove it

Removing Longhorn deletes its volumes and their data. Back up first, and move every workload off Longhorn volumes. Longhorn refuses to uninstall until you confirm it:

```sh
kubectl -n longhorn-system patch -p '{"value": "true"}' --type=merge lhs deleting-confirmation-flag
```

The `longhorn-static` storage class stays, because Longhorn creates it without an owner. Delete it by hand. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A node is not Ready

Run `kubectl -n longhorn-system get nodes.longhorn.io`. The node's Ready condition names what the node lacks, such as open-iscsi, an NFS client or a kernel module. Install it on that node.

### A volume stays degraded or won't attach

Check the Pods with `kubectl -n longhorn-system get pods`. Then read `kubectl -n longhorn-system get volumes.longhorn.io`. A cluster with fewer nodes than copies runs the volume with fewer copies until more nodes join.
