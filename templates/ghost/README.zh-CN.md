## 简介

[Ghost](https://github.com/TryGhost/Ghost) 是用来运营博客和付费邮件通讯的发布平台。这个模板在你的域名后面运行 Ghost 6，MySQL 放在卷上。

Ghost 通过你在安装表单填写的 SMTP 服务器发送登录邮件和系统邮件。

## 开始使用

1. 在 Zeabur 打开 `ghost` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `blog.example.com`。管理后台在这个域名的 `/ghost`。
2. 设置任务会用你在安装表单填的所有者邮箱创建站点所有者，所以不用再填设置表单。获取所有者密码。在 `ghost` 服务打开**变量**，在**来自配置文件和机密变量**中点击 `ghost-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `ghost-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret ghost-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 打开管理后台，用所有者邮箱和这个密码登录。

`ghost-secrets` 里其他的字段（`database-password` 和 `root-password`）是给 MySQL 用的。Ghost 不会用到 `root-password`。

## 配置

### 发送邮件通讯

SMTP 只负责系统邮件。要把文章作为邮件通讯发出去，Ghost 需要批量发信服务，目前只支持 Mailgun。在管理后台打开邮件通讯的邮件设置，输入你的 Mailgun API 密钥。详见 [Newsletters](https://docs.ghost.org/newsletters) 和 [Why do I have to set up Mailgun?](https://docs.ghost.org/faq/mailgun-newsletters)。

## 故障排除

### 登录邮件或系统邮件发不出去，出现连接超时

有些主机会封锁对外的 SMTP 端口。请改用你的邮件服务和网络都允许的 SMTP 端口。详见 [Configuration](https://docs.ghost.org/config)。
