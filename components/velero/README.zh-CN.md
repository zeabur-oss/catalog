## 简介

Velero 把 Kubernetes 对象和卷里的文件备份到 S3 兼容存储，也能还原。想备份、还原整个项目时就装它。数据库备份不需要它。

Velero 绑定 `cluster-admin`，能读取并还原集群里所有的对象，所以只给管理员使用。默认没有任何 Pod 是特权模式。CSI 快照是关闭的。Velero 用 Kopia 复制卷里的文件，要开启 `fileBackup` 输入才会复制。Velero 会把 Secret 原样存进备份，所以安装时会生成两个密钥。一个是 Kopia 仓库密码，另一个让 S3 服务器加密每个备份对象（SSE-C）。

## 安装前准备

先准备一个集群外的 S3 存储桶。备份放在集群里的话，集群一坏，备份也跟着没了。然后备齐这些输入：

- `bucket`：存储桶名称。
- `s3Url`：S3 端点地址。要用 HTTPS，存储服务也要支持 SSE-C。
- `region`：存储桶所在区域，默认为 `us-east-1`。
- `credentials`：存储桶的密钥，格式是 AWS 风格的凭证文件，在 `[default]` 配置段里写 `aws_access_key_id` 和 `aws_secret_access_key`。
- `fileBackup`：不需要备份卷文件就保持 `false`。设为 `true` 时，每个节点会以 root 运行一个节点代理，挂载主机路径并持有 Velero 的令牌，`velero` Namespace 也必须允许特权（privileged）Pod 安全级别。

## 使用方法

在 Zeabur 开源版打开**集群** → **备份**，配置备份存放位置和定时、运行备份与还原。详见[备份集群并从备份还原](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/backups/)。

这两个密钥也存在它们保护的备份里。现在就复制到集群外。没有密钥，就没有人能读取备份。

```sh
kubectl -n velero get secret velero-repo-credentials velero-backup-key -o yaml > velero-keys.yaml
```

## 移除

移除 Velero 会删除它的 Deployment，以及你开启过的节点代理。备份和还原会停止。CRD，以及 Backup、Schedule、Restore 对象会留在集群里。存储桶里的备份和生成的密钥也都会保留。请保管好密钥，读取旧备份时要用。移除前先让运行中的备份跑完。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 备份存放位置连不上

用 `kubectl -n velero get backupstoragelocations` 和 `kubectl -n velero describe backupstoragelocation default` 查看它的状态和信息。再检查存储桶、端点、区域和密钥。Velero 的日志用 `kubectl -n velero logs deploy/velero` 查看。

### 备份结果是 PartiallyFailed

`fileBackup` 为 `false` 时，要求复制卷文件的备份会因为复制不了而部分失败。运行 `kubectl -n velero get backups`，再用 `velero backup describe <backup> --details` 查看失败的项目。把组件的 `fileBackup` 设为 `true`，再备份一次。
