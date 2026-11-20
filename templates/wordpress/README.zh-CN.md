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
