## 简介

MariaDB Operator 依据 `MariaDB` 对象运行 MariaDB，可单机也可复制。用户、权限和数据库也都是对象，备份会存到 S3。项目需要开源许可、与 MySQL 兼容的数据库时就装它。Operator 采用 MIT 许可，它运行的 MariaDB Server 镜像则是 GPL-2.0。

这个组件会在 `mariadb-operator` Namespace 安装两个 release。一个放 CRD，让 CRD 先于 Operator 升级；另一个放 Operator 和它的 webhook。Webhook 的证书要靠 cert-manager 签发。集群还没有 cert-manager 时，安装对话框会先装它。Operator 会为每个 Namespace 的 `MariaDB` 对象管理 Pod、Service、Secret、PVC、Job 和 RBAC。

## 使用方法

在 Zeabur 打开项目的**新建**菜单，选择 **MariaDB**。设置版本和存储空间，再连接到服务，并在数据库的**备份**标签页配置 S3 备份。详见[创建、备份与还原数据库](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/)。

## 移除

移除组件会删除 Operator、它的 webhook 和 CRD。删除 CRD 会连带删除所有 `MariaDB`、备份、用户和权限对象。有这些对象时 Zeabur 会拒绝移除，所以先删除不要的数据库，其余的先备份。PVC 会保留，确定不再需要数据时，自己删除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 数据库一直没有就绪

用 `kubectl -n <namespace> describe mariadb <name>` 查看对象和它的事件，再用 `kubectl -n <namespace> get pods` 列出它的 Pod。Operator 的日志用 `kubectl -n mariadb-operator logs deploy/mariadb-operator` 查看。

### API 服务器因 webhook 错误拒绝 `MariaDB` 对象

用 `kubectl -n mariadb-operator get pods` 确认 webhook 的 Pod 在运行，再用 `kubectl -n mariadb-operator get certificate mariadb-operator-webhook-cert` 确认它的证书已就绪。
