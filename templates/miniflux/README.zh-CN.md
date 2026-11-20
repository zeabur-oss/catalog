## 简介

[Miniflux](https://miniflux.app/) 是简约的 RSS 阅读器。它会抓取你订阅的 RSS 和 Atom 订阅源，把新文章集中在一个地方。

这个模板在你的域名后面运行一个 Miniflux，并附带专属的 PostgreSQL 数据库。第一次启动时会建好名为 `admin` 的管理员账号。

## 开始使用

1. 在 Zeabur 打开 `miniflux` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，就是你在安装表单填的域名，例如 `miniflux.example.com`。
2. 以 `admin` 登录。密码从 `miniflux` 服务获取：打开**变量**，在**来自配置文件和机密变量**中点击 `miniflux-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `miniflux-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret miniflux-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 添加第一个订阅源。
4. 需要的话可以在 Miniflux 里改密码。Miniflux 只在创建账号时读取这个机密，之后改机密不会改变密码。

## 配置

### 创建 API 密钥

前往 Settings，再到 API Keys，选 Create a new API key。把密钥放在 `X-Auth-Token` 请求头发送：

```sh
curl -H "X-Auth-Token: <key>" https://miniflux.example.com/v1/me
```

详见 Miniflux 文档的 [API](https://miniflux.app/docs/api.html)。

### 使用阅读器应用

许多阅读器应用可以通过 Google Reader API 或 Fever API 连接 Miniflux。在 Miniflux 里打开 Settings，再到 Integrations，为 API 选一组用户名和密码。在应用里填入这组用户名和密码。支持的应用请见 Miniflux 文档的 [Google Reader API](https://miniflux.app/docs/google_reader.html) 和 [Fever API](https://miniflux.app/docs/fever.html)。

### 备份数据

Miniflux 把订阅源、文章和设置存在 `miniflux-db` 数据库，没有往卷里放任何东西。在项目中打开 `miniflux-db` 数据库。到**备份**配置 S3 备份。想直接拿到文件，点击**更多** → **导出**。详见 Zeabur 文档的[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和[下载数据库转储文件](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#下载数据库转储文件)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `miniflux` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

详见 Zeabur 文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

### 私有地址的订阅源被拒绝

Miniflux 默认不会连接私有和本机回环的网络地址，模板也没有改动这项设置。请改用公共互联网能访问到的订阅源地址。详见 [Configuration parameters](https://miniflux.app/docs/configuration.html) 里的 `FETCHER_ALLOW_PRIVATE_NETWORKS`。

### 手动刷新没有立刻更新

Miniflux 默认每 30 分钟才允许手动刷新一次（`FORCE_REFRESH_INTERVAL`），其余时间由它在后台按自己的计划更新订阅源。详见 [Configuration parameters](https://miniflux.app/docs/configuration.html)。
