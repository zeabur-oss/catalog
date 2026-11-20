## 简介

SeaweedFS Operator 依据 `Seaweed` 对象运行 SeaweedFS 对象存储，并提供 S3 API。存储桶、用户、密钥和策略也都是对象。项目需要集群内的 S3 对象存储时就装它。

这个组件目前为 Beta。chart 大多出自单一维护者，功能和 CRD 在版本之间仍会变动。Operator 的 webhook 证书要靠 cert-manager 签发。

Operator 会为每个 Namespace 的 `Seaweed` 对象管理 Pod、StatefulSet、Service、Secret、PVC、Job 和 Ingress，也会调用各存储服务的管理 API。Grafana 仪表板是关闭的。

## 使用方法

在 Zeabur 打开项目的**新建**菜单，选择**对象存储**。设置容量和第一个存储桶，再把 S3 端点和密钥连接到服务。详见[创建 S3 对象存储与存储桶](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/object-storage/)。

## 移除

移除 Operator 只会删除 Operator 本身和 webhook。CRD 会保留，你的 `Seaweed` 和存储桶对象、Operator 创建的 Pod 和它们的 PVC 也都会保留。在 Operator 装回来之前，没有人管理这些对象。只要还有 `Seaweed` 对象，Zeabur 就拒绝移除这个组件。请先在 Zeabur 删除对象存储，还需要的文件先备份。已删除存储服务留下的 PVC 要自己删除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 存储服务一直没有就绪

Operator 会在 master、volume 和 filer 之后才创建 S3 网关。用 `kubectl -n <namespace> get seaweed` 和 `kubectl -n <namespace> get pods` 查看进度，再用 `kubectl -n seaweedfs-operator logs deploy/seaweedfs-operator` 查看 Operator 的日志。

### `Seaweed` 对象被 webhook 拒绝

Webhook 需要证书。用 `kubectl -n seaweedfs-operator get certificate seaweedfs-operator-webhook-cert` 检查，并确认 cert-manager 在运行。
