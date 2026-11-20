## 简介

MOCO 依据 `MySQLCluster` 对象运行 MySQL，可以是单个实例，也可以是半同步的主节点加副本。MOCO 会备份到 S3，并能还原到某个时间点。应用需要 MySQL 本身时就装它。Operator 采用 Apache-2.0 许可，运行的 MySQL 镜像则是 GPL-2.0。

MOCO 需要 cert-manager 签发证书，用于 webhook，以及控制器与代理之间的 mTLS。集群还没有 cert-manager 时，安装对话框会先装它。控制器会为每个 Namespace 的 `MySQLCluster` 对象管理 Pod、StatefulSet、Service、Secret、PVC、CronJob、Job 和 Role，并把证书复制进去。

## 使用方法

在 Zeabur 打开项目的**新建**菜单，选择 **MySQL**。设置版本和存储空间，再连接到服务，并在数据库的**备份**标签页配置 S3 备份。详见[创建、备份与还原数据库](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/)。

## 移除

移除 MOCO 会移除控制器和它的 webhook。只要还有 `MySQLCluster` 对象，Zeabur 就拒绝移除，所以先删除不要的数据库，其余的先备份。CRD 会保留，已删除数据库的 PVC 也会保留。确定不再需要数据时，自己删除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 数据库一直不是 Healthy

用 `kubectl -n <namespace> get mysqlcluster <name>` 查看对象，再用 `kubectl -n <namespace> get pods` 查看它的 Pod。控制器的日志用 `kubectl -n moco-system logs deploy/moco-controller` 查看。

### API 服务器因 webhook 错误拒绝 `MySQLCluster` 对象

用 `kubectl -n moco-system get pods` 确认控制器在运行，再用 `kubectl -n moco-system get certificate` 确认它的证书已就绪。
