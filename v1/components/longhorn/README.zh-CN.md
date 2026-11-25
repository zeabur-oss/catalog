## 简介

Longhorn 是块存储，会把每个卷复制到其他节点。节点故障时卷仍在，会跟着 Pod 换到别的节点，也能原地扩容。多节点集群建议安装，它会成为默认的存储类。单节点集群可选装：副本没有其他节点可去，但仍能用到可扩容的卷和快照。KubeSolo 不能装。

这个组件让每个卷最多有三个副本，每个节点放一个，有几个节点就放几个。节点比副本少时，卷照样能挂载，节点加入后再补上缺少的副本。单节点时，默认存储类仍是 `local-path`，`longhorn` 类只要一个副本。

Longhorn 在每个节点运行特权 Pod。这些 Pod 会从主机挂载 `/dev`、`/proc`、`/boot`、`/etc` 和 `/var/lib/longhorn`，并管理整个集群的 Pod、PersistentVolume 和存储类。Longhorn UI 没有登录验证，也没有 Ingress，而且 NetworkPolicy 不让其他 Pod 连到。

## 安装前准备

先把每个节点准备好：

- open-iscsi，`iscsid` 要在运行，`iscsi_tcp` 模块要加载。
- NFSv4 客户端，供 ReadWriteMany 卷和 NFS 备份使用。
- `bash`、`curl`、`findmnt`、`grep`、`awk`、`blkid` 和 `lsblk`。
- 如果运行了 `multipathd`，要把 Longhorn 的磁盘加入黑名单，见 [Longhorn 的 multipath 说明](https://longhorn.io/kb/troubleshooting-volume-with-multipath/)。

在节点上运行 `longhornctl check preflight` 可以检查。安装后，`kubectl -n longhorn-system get nodes.longhorn.io` 也会在各节点的状况中显示同样的结果。

## 使用方法

在 Zeabur 开源版添加卷时，在**存储类**中选择**多副本磁盘**。详见[选择存储类](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/volumes/#选择存储类)。在 Zeabur 开源版之外，把 PVC 的 `storageClassName` 设为 `longhorn`：

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

要打开 Longhorn UI，运行 `kubectl -n longhorn-system port-forward svc/longhorn-frontend 8080:80`，再访问 `http://localhost:8080`。详见 [Longhorn 文档](https://longhorn.io/docs/1.13.0/)。

## 移除

移除 Longhorn 会删除所有卷和里面的数据。先备份，并让所有工作负载离开 Longhorn 卷。Longhorn 要先确认才会卸载：

```sh
kubectl -n longhorn-system patch -p '{"value": "true"}' --type=merge lhs deleting-confirmation-flag
```

`longhorn-static` 存储类会保留，因为 Longhorn 创建它时没有设置所有者，要自己删除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 节点不是 Ready

运行 `kubectl -n longhorn-system get nodes.longhorn.io`。节点的 Ready 状况会写出它缺什么，例如 open-iscsi、NFS 客户端或内核模块。到那个节点上补装缺少的项目。

### 卷一直是降级状态，或挂载不上

用 `kubectl -n longhorn-system get pods` 检查 Pod，再查看 `kubectl -n longhorn-system get volumes.longhorn.io`。节点比副本少的集群，在更多节点加入前，卷会用较少的副本运行。
