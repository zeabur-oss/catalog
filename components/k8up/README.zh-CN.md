## 简介

K8up 用 restic 把 PersistentVolumeClaim 的内容备份到 S3 兼容存储。服务需要备份自己的卷时就装它。K8up 备份的是单个服务的文件，不是整个集群，数据库备份不需要它。

K8up 只备份带有 `k8up.io/backup=true` 注解的 PVC，所以装好后不会自己备份任何东西。控制器会监视 K8up 对象，并读取每个 Namespace 的 PVC、PersistentVolume 和 Pod，也会在卷所在的 Namespace 创建备份 Job、ServiceAccount 和 RoleBinding。每个备份 Job 只挂载自己 Namespace 里的卷。

## 使用方法

在 Zeabur 打开服务的**设置** → **存储**，在下方的**备份**中找到卷，点击**备份这个卷**。先备好 S3 存储桶和密钥，或请管理员登记 S3 目的地。Zeabur 会创建 K8up Schedule，以及存放 restic 密码的 Secret。详见[配置卷备份](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/volumes/#配置卷备份)。

把备份 Secret 里的 `RESTIC_PASSWORD` 另外保存到安全的地方。没有它，就读不了 S3 里的备份。

## 移除

移除 K8up 后，所有定时备份都会停止。chart 被移除时，清理 Job 会删除 K8up 在每个 Namespace 添加的 `pod-executor` Role 和 RoleBinding。CRD 会保留，Schedule 对象、备份 Secret 和 S3 存储桶里的数据也都会保留。在 K8up 装回来之前，Schedule 不会做任何事。要永久停止某个备份，用 `kubectl -n <namespace> delete schedule.k8up.io <name>` 删除调度。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 备份没有开始或失败

确认 PVC 带有 `k8up.io/backup=true`：`kubectl -n <namespace> get pvc <name> -o jsonpath='{.metadata.annotations}'`。再用 `kubectl -n <namespace> get backups.k8up.io` 列出运行记录，并用 `kubectl -n k8up-system logs deploy/k8up` 查看 operator 的日志。
