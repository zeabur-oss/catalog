## 简介

[Umami](https://umami.is/) 是网站流量分析的仪表盘。你在网页上加一小段跟踪脚本，就能在仪表盘看到访问数据。

这个模板在你的域名后面运行一个 Umami，并附带它自己的 PostgreSQL 数据库（Service `umami-db-rw`，端口 `5432`），由 CloudNativePG 运算符管理。它在项目里显示为数据库 `umami-db`。

## 开始使用

1. 在 Zeabur 打开 `umami` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**，也就是你在安装表单填的域名。
2. 用 `admin` 登录。打开**变量**，在**来自配置文件和机密变量**点 `umami-secrets`，再点 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `umami-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret umami-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

   模板在第一次启动时，已经替换掉公开的默认密码。登录后，请在 Umami 里更改这个密码。
3. 添加你的网站，再把跟踪脚本放进网站的页面（见“配置”）。

同一个机密变量还有 `app-secret`，用来签署登录令牌和分享链接。使用 Umami 时用不到。

## 配置

### 添加网站并安装跟踪代码

点侧边栏的 Websites，再点 Add website，输入名称和域名后保存。点该网站的 Edit，复制 Tracking code 下面的代码，放进页面的 `<head>` 部分。详见 [Add a website](https://docs.umami.is/docs/add-a-website) 和 [Collect data](https://docs.umami.is/docs/collect-data)。

### 更改你的密码

点侧边导航栏的个人资料按钮，依次进入 Settings、Security，再点 Change password。Umami 3.5.0 之前，这个按钮在 Profile 页面，不在 Security。详见 [Login](https://docs.umami.is/docs/login)。

### 备份数据

在 Zeabur 打开 `umami-db` 数据库，到**备份** → **设置备份**，就能按计划备份到 S3 存储桶。PostgreSQL 备份需要 Barman Cloud 插件。要现在就拿一份数据，在数据库页面右上角点击**更多** → **导出**，浏览器会下载 `pg_dump --format=custom` 生成的 `.dump` 文件。详见[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和[下载数据库转储文件](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#下载数据库转储文件)。

Umami 的数据都在这个数据库里，所以转储文件就是完整备份。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `umami` 服务，确认**概览**显示副本已就绪，再看**日志**选项卡。副本一直重启时，打开**部署记录**，切换到**事件**。如果事件显示 `OOMKilled`，到**设置** → **计算资源**调高**内存上限**。详见[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

域名或证书一直处于等待中时，看[添加公开域名与 HTTPS](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)的“遇到问题”。

### 数据没有出现

在你的网站上打开浏览器开发者工具，在 Network 标签页找出发往你的 Umami 域名的请求，确认脚本加载时没有错误。广告拦截器可能让脚本无法加载。详见 [Collect data](https://docs.umami.is/docs/collect-data)。
