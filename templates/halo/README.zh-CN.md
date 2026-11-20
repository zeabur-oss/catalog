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

### 备份站点

在后台打开“备份”，点“创建备份”。Halo 会在后台进行备份。备份文件放在 Halo 工作目录的 `backups` 文件夹，也就是卷 `halo-data` 上。请另外在别处保留一份。详见 [备份与恢复](https://docs.halo.run/guide/use/backup)。
