## 简介

[WordPress](https://wordpress.org/) 是网站和博客引擎。这个模板在你的域名后面运行 WordPress，并附带它自己的 MariaDB 数据库。网站文件和上传的文件存在卷里，数据库则有自己的 10Gi 卷。

网站已经安装好。所有者账号 `admin` 已建好，管理员邮箱就是你在安装表单填的那一个。允许上传最大 256 MB 的文件。

## 开始使用

1. 在 Zeabur 打开 `wordpress` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**，也就是你在安装表单填的域名。
2. 在 `https://<你的域名>/wp-admin` 用 `admin` 登录。打开**变量**，在**来自配置文件和机密变量**点 `wordpress-secrets`，再点 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `wordpress-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 数据库是 MariaDB，在项目里是 `mariadb` 数据库。WordPress 连到 Service `mariadb` 的 `3306` 端口，数据库是 `zeabur`，用户是 `wordpress`，连接使用 TLS。密码在同一个机密变量的 `database-password` 和 `database-root-password`。用同样的方式打开，或用 kubectl 读取：

   ```sh
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.database-password}' | base64 -d
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.database-root-password}' | base64 -d
   ```

4. 同一个机密变量还有 WordPress 用来签署登录 cookie 的密钥和 salt：`auth-key`、`secure-auth-key`、`logged-in-key`、`nonce-key`、`auth-salt`、`secure-auth-salt`、`logged-in-salt` 和 `nonce-salt`。使用网站时用不到。要读取时用同样的命令，例如 `{.data.auth-key}`。

## 配置

### 更改管理员密码

登录后，进入 Users、All Users，点你的用户名。在 New Password 下点 Generate Password，或自己输入新密码，再点 Update User。新密码立刻生效。`wordpress-secrets` 里的值只在第一次安装时使用，所以不会跟着改变。详见 [Reset your password](https://wordpress.org/documentation/article/reset-your-password/)。

### 选择主题和添加插件

进入 Appearance、Themes 更换网站外观，进入 Plugins 添加功能。详见 [First steps with WordPress](https://wordpress.org/documentation/article/first-steps-with-wordpress/)。

### 设置页面的网址格式

进入 Settings、Permalinks，选一种结构，再点 Save Changes。详见 [Settings Permalinks screen](https://wordpress.org/documentation/article/settings-permalinks-screen/)。

### 备份数据

在 Zeabur 打开 `mariadb` 数据库，到**备份** → **设置备份**，就能按计划备份到 S3 存储桶。要现在就拿一份数据，在数据库页面右上角点击**更多** → **导出**，浏览器会下载 `mariadb-dump --single-transaction` 生成、gzip 压缩的 SQL。详见[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和[下载数据库转储文件](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#下载数据库转储文件)。

上传的文件、主题和插件不在数据库里，而是在 `wordpress-data` 卷上，挂载在 `wordpress` 服务的 `/var/www/html`。请把 `wp-content` 和数据库转储文件一起备份。在 Zeabur 可以用**文件**选项卡下载文件夹，但文件夹很大时可能超时。`wp-content` 很大时，改用 kubectl 流式输出：

```sh
kubectl -n <namespace> exec deploy/wordpress -c wordpress -- tar czf - -C /var/www/html wp-content > wp-content.tar.gz
```

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `wordpress` 服务，确认**概览**显示副本已就绪，再看**日志**选项卡。副本一直重启时，打开**部署记录**，切换到**事件**。如果事件显示 `OOMKilled`，到**设置** → **计算资源**调高**内存上限**。详见[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

域名或证书一直处于等待中时，看[添加公开域名与 HTTPS](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)的“遇到问题”。
