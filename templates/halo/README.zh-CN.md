## 简介

[Halo](https://github.com/halo-dev/halo) 是建站与博客系统，有主题和插件。这个模板在你的域名后面运行 Halo，搭配 PostgreSQL 数据库。

## 开始使用

1. 在 Zeabur 打开 `halo` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `halo.example.com`。访客在这里看到站点，管理后台在 `/console`。
2. 第一次启动时会创建超级管理员 `admin`，不会出现初始化向导。获取它的密码。打开**变量**，在**来自配置文件和机密变量**中点击 `halo-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `halo-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret halo-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 打开 `https://halo.example.com/console`，用 `admin` 和这个密码登录，再到后台修改密码。

## 配置

### 安装主题

在后台打开“主题”，再点右上角的“主题管理”。你可以从内置的应用市场安装主题、上传主题压缩包，或通过链接下载。安装后选中主题，再点“启用”。详见 [主题](https://docs.halo.run/guide/use/themes)。

### 备份数据

Halo 有内置的备份。在后台打开“备份”，点“创建备份”。Halo 会在后台进行备份。备份文件放在 Halo 工作目录的 `backups` 文件夹，也就是卷 `halo-data`（`/var/lib/halo`）上。请另外在别处保留一份。

Halo 把内容存在 `halo-db` 数据库。在项目中打开 `halo-db`，到**备份**配置 S3 备份。想直接拿到文件，点击**更多** → **导出**。

很大的备份文件在浏览器里可能上传失败。改用 kubectl 复制到 `backups` 文件夹，再从后台的备份文件恢复：

```sh
kubectl -n <namespace> cp ./<backup file> halo-0:/var/lib/halo/backups/
```

详见 Halo 文档的[备份与恢复](https://docs.halo.run/guide/use/backup)。详见 Zeabur 文档的[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和[下载数据库转储文件](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#下载数据库转储文件)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `halo` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

不用 Zeabur 时，用 `kubectl -n <namespace> logs statefulset/halo` 查看日志，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 查看事件。

详见 Zeabur 文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。
