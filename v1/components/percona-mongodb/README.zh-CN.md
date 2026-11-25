## 简介

Percona Operator for MongoDB 依据 `PerconaServerMongoDB` 对象运行 MongoDB 副本集，并用 Percona Backup for MongoDB 定时备份到 S3。项目需要 MongoDB 时就装它。

Operator 采用 Apache-2.0 许可。它运行的 Percona Server for MongoDB 镜像采用 SSPL-1.0，这不是 OSI 认可的开源许可。使用前请先阅读 SSPL。

Operator 会为每个 Namespace 的 MongoDB 集群管理 Pod、Service、Secret 和 PVC。这个组件关闭了 Operator 的遥测（默认会上报给 Percona）。

## 使用方法

在 Zeabur 开源版打开项目的**新建**菜单，选择 **MongoDB**。设置版本和存储空间，再连接到服务，并在数据库的**备份**标签页配置 S3 备份。详见[创建、备份与还原数据库](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/)。

## 移除

移除 Operator 后，CRD 会保留。只要还有 `PerconaServerMongoDB` 对象，Zeabur 开源版就拒绝移除，所以先删除不要的数据库，其余的先备份。已删除数据库的 PVC 会保留。确定不再需要数据时，自己删除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 数据库一直没有就绪

用 `kubectl -n <namespace> get psmdb <name>` 查看对象，再用 `kubectl -n <namespace> get pods` 查看它的 Pod。Operator 的日志用 `kubectl -n psmdb-operator logs deploy/psmdb-operator` 查看。

### MongoDB 8 的 Pod 一启动就崩溃

MongoDB 8 无法在 Linux 6.19 及以上启动（SERVER-121912）。用 `uname -r` 查看节点的内核版本。Zeabur 开源版创建的 MongoDB 8 数据库都会添加 `GLIBC_TUNABLES=glibc.pthread.rseq=1`。自己创建的数据库请改用 7.0 镜像。
