## 简介

[Gitea](https://about.gitea.com/) 是 Git 托管服务，提供仓库、议题和合并请求。这个模板在你的域名后面运行 Gitea，仓库放在卷上，搭配 PostgreSQL 数据库。

公开注册已关闭，SSH 也关闭了。大家用 HTTPS 使用 Gitea。

## 开始使用

1. 在 Zeabur 开源版打开 `gitea` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `git.example.com`。不会出现安装向导。
2. 第一次启动时会创建管理员 `gitea_admin`。获取它的密码。打开**变量**，在**来自配置文件和机密变量**中点击 `gitea-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 开源版时，用 kubectl 读取机密变量 `gitea-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret gitea-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 用 `gitea_admin` 和这个密码登录，并在 Gitea 里修改密码。Gitea 只在第一次启动时读取这个机密，之后再改机密不会生效。

`gitea-secrets` 里其他的字段（`secret-key`、`internal-token`、`jwt-secret` 和 `lfs-jwt-secret`）是 Gitea 自己使用。`secret-key` 要在整个数据库的使用期间保留，因为 Gitea 用它加密双因素验证密钥等已保存的凭证。

## 配置

### 创建用户

注册已关闭，其他用户由管理员创建。打开 Site Administration，再打开 User Accounts。详见 Gitea 的 [FAQ](https://docs.gitea.com/help/faq)。

### 用访问令牌通过 HTTPS 推送

在 Settings > Applications 创建访问令牌，也可以直接打开 `https://git.example.com/user/settings/applications`。给它需要的权限，并在 Gitea 显示时复制下来，因为 Gitea 只显示一次。Git 要求密码时，把令牌当作密码输入：

```sh
git clone https://git.example.com/<用户>/<仓库>.git
```

详见 [API Usage](https://docs.gitea.com/development/api-usage)。

### 备份数据

Gitea 把账号、议题和设置存在 `gitea-db` 数据库。在项目中打开 `gitea-db`，到**备份**配置 S3 备份。想直接拿到文件，点击**更多** → **导出**。仓库和上传的文件放在卷 `gitea-data` 上，挂载在 `/var/lib/gitea`。用 `gitea` 服务的**文件**标签页复制。文件夹很大时，改用 kubectl。`kubectl -n <namespace> get pods` 会列出副本名称：

```sh
kubectl -n <namespace> cp <replica>:/var/lib/gitea ./gitea-data
```

Gitea 也有 `gitea dump` 命令，把仓库、数据库和数据打包成一个 ZIP 文件。Gitea 建议备份期间先停止实例，让文件和数据库保持一致。详见 Zeabur 开源版文档的[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)、[下载数据库转储文件](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#下载数据库转储文件)，以及 Gitea 文档的 [Backup and Restore](https://docs.gitea.com/administration/backup-and-restore)。

## 故障排除

### 用 SSH 克隆仓库失败

这个模板关闭了 SSH。请使用仓库的 HTTPS 克隆地址。详见 [Config Cheat Sheet](https://docs.gitea.com/administration/config-cheat-sheet)。

### 页面显示 502 或打不开

在 Zeabur 开源版打开 `gitea` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

不用 Zeabur 开源版时，用 `kubectl -n <namespace> logs deployment/gitea` 查看日志，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 查看事件。

详见 Zeabur 开源版文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。
