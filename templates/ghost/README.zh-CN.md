## 简介

[Ghost](https://github.com/TryGhost/Ghost) 是用来运营博客和付费邮件通讯的发布平台。这个模板在你的域名后面运行 Ghost 6，搭配由 MOCO 运行的 MySQL 数据库。

Ghost 通过你在安装表单填写的 SMTP 服务器发送登录邮件和系统邮件。

## 开始使用

1. 在 Zeabur 开源版打开 `ghost` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `blog.example.com`。管理后台在这个域名的 `/ghost`。
2. 设置任务会用你在安装表单填的所有者邮箱创建站点所有者，所以不用再填设置表单。获取所有者密码。在 `ghost` 服务打开**变量**，在**来自配置文件和机密变量**中点击 `ghost-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 开源版时，用 kubectl 读取机密变量 `ghost-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret ghost-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 打开管理后台，用所有者邮箱和这个密码登录。

机密 `ghost-mysql-app` 的 `password` 是 Ghost 连接时用的 MySQL 用户 `ghost` 的密码。MOCO 自己的管理员密码放在 `moco-ghost-mysql`。

## 配置

### 发送邮件通讯

SMTP 只负责系统邮件。要把文章作为邮件通讯发出去，Ghost 需要批量发信服务，目前只支持 Mailgun。在管理后台打开邮件通讯的邮件设置，输入你的 Mailgun API 密钥。详见 [Newsletters](https://docs.ghost.org/newsletters) 和 [Why do I have to set up Mailgun?](https://docs.ghost.org/faq/mailgun-newsletters)。

### 备份数据

Ghost 把文章、会员和设置存在 `ghost-mysql` 数据库。在项目中打开 `ghost-mysql`，到**备份**配置 S3 备份。想直接拿到文件，点击**更多** → **导出**。图片、主题和其他上传的文件放在卷 `ghost-data` 上，挂载在 `/var/lib/ghost/content`。

内容文件夹用 `ghost` 服务的**文件**标签页复制。文件夹很大时，改用 kubectl：

```sh
kubectl -n <namespace> cp ghost-0:/var/lib/ghost/content ./ghost-content
```

Ghost 后台的“设置 > 高级 > 导入/导出”可以把文章和设置导出为 JSON，但自托管站点的图片不在其中。详见 Zeabur 开源版文档的[创建、备份与还原数据库](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和 Ghost 文档的 [Exporting content and data](https://ghost.org/help/exports/)。

### 更换数据库密码

Ghost 从 `ghost-mysql-app` 读取密码，但 MySQL 自己也存了一份。只改机密的话，Ghost 会连不上数据库。要先以 `moco-writable` 在 MySQL 里改：

```sh
kubectl -n <namespace> exec -i moco-ghost-mysql-0 -c mysqld -- mysql -h127.0.0.1 -umoco-writable \
  -p"$(kubectl -n <namespace> get secret moco-ghost-mysql -o jsonpath='{.data.WRITABLE_PASSWORD}' | base64 -d)" \
  -e "ALTER USER 'ghost'@'%' IDENTIFIED BY '<new password>'"
```

接着在 `ghost-mysql-app` 里为 `password` 填入同一个值，检查后应用，再点击 `ghost` 服务页面顶部的**重启**。

## 故障排除

### 安装后 Ghost 一直重启

MOCO 启动 MySQL、任务 `ghost-mysql-app-setup` 建好 `ghost` 用户之前，Ghost 连不上数据库。这需要几分钟，在那之前 Ghost 会一直重启。用 `kubectl -n <namespace> get mysqlcluster ghost-mysql` 查看数据库，用 `kubectl -n <namespace> get job ghost-mysql-app-setup` 查看任务。

### 登录邮件或系统邮件发不出去，出现连接超时

有些主机会封锁对外的 SMTP 端口。请改用你的邮件服务和网络都允许的 SMTP 端口。详见 [Configuration](https://docs.ghost.org/config)。

### 页面显示 502 或打不开

在 Zeabur 开源版打开 `ghost` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。服务变慢或一直重新连接时，打开**用量**标签页，出现**接近内存上限**就用同样的方式调高上限。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

不用 Zeabur 开源版时，用 `kubectl -n <namespace> logs statefulset/ghost` 查看日志，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 查看事件。

详见 Zeabur 开源版文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。
