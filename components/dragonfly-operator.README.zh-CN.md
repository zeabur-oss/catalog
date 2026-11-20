## 简介

Dragonfly Operator 依据 `Dragonfly` 对象运行 Dragonfly（兼容 Redis 的内存数据库）：一个主节点加副本、故障切换，以及快照到卷。项目需要兼容 Redis 的数据库时就装它。

Operator 采用 Apache-2.0 许可。它运行的 Dragonfly 镜像采用 BUSL-1.1，属于源码可得（source-available）许可，不是 OSI 认可的开源许可。使用前请先阅读许可的限制，尤其是向他人提供托管数据库服务的场景。

Operator 会为每个 Namespace 的 `Dragonfly` 对象管理 Pod、StatefulSet、Service 和 PodDisruptionBudget。`kube-rbac-proxy` 容器负责保护它的指标端口。

## 使用方法

在 Zeabur 打开项目的**新建**菜单，选择 **Dragonfly**。设置存储空间，再连接到服务。Dragonfly 没有**备份**标签页，快照存在自己的卷上，启动时加载最新的一份。详见[创建、备份与还原数据库](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/)。

## 移除

移除 Operator 后，CRD 和所有 `Dragonfly` 对象都会保留。在 Operator 装回来之前，没有人管理这些对象。只要还有 `Dragonfly` 对象，Zeabur 就拒绝移除这个组件。请先删除不要的数据库。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 数据库一直没有就绪

用 `kubectl -n <namespace> get dragonfly <name>` 查看对象，再用 `kubectl -n <namespace> get pods` 查看它的 Pod。Operator 的日志用 `kubectl -n dragonfly-operator-system logs deploy/dragonfly-operator -c manager` 查看。
