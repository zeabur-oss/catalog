## 简介

CloudNativePG 依据 `Cluster` 对象运行 PostgreSQL，支持副本、故障切换和备份到 S3。项目需要 PostgreSQL 数据库时就装它。这个组件会在 `cnpg-system` Namespace 安装两个 release：Operator，以及负责写入备份的 Barman Cloud 插件。

Operator 需要 cert-manager，用证书保护 Operator 和插件之间的连接。集群还没有 cert-manager 时，安装对话框会先装它。Operator 会为每个 Namespace 的 `Cluster` 对象管理 Pod、Service、Secret 和 PVC。PodMonitor 和 Grafana 仪表板是关闭的。

## 使用方法

在 Zeabur 打开项目的**新建**菜单，选择 **PostgreSQL**。设置版本和存储空间，再连接到服务，并在数据库的**备份**标签页配置 S3 备份。详见[创建、备份与还原数据库](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/)。

CloudNativePG 会把备份文件以明文写进存储桶。请使用开启服务端加密的存储桶，并限制谁能读取。

## 移除

移除组件会移除 Operator 和插件。只要还有 `Cluster` 对象，Zeabur 就拒绝移除，所以先删除不要的数据库，其余的先备份。CRD 会保留，已删除数据库的 PVC 也会保留。确定不再需要数据时，自己删除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 数据库一直没有就绪

用 `kubectl -n <namespace> describe cluster <name>` 查看对象和它的事件，再用 `kubectl -n <namespace> get pods` 列出它的 Pod。Operator 的日志用 `kubectl -n cnpg-system logs deploy/cnpg-cloudnative-pg` 查看。

### 备份没有开始

用 `kubectl -n cnpg-system get pods` 确认插件在运行，再用 `kubectl -n cnpg-system get certificate` 确认它的证书已就绪。插件需要 `barman-cloud-client` 和 `barman-cloud-server` 两张证书。
