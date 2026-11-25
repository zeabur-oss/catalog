## 简介

[OpenList](https://github.com/OpenListTeam/OpenList) 把多个存储服务的文件列在同一个网页里，也能通过 WebDAV 提供。这个模板使用一体式镜像，内含 FFmpeg 和 Aria2。

这个模板在你的域名后面运行一个 OpenList。管理员密码和 Aria2 密码都在部署时自动生成。

## 开始使用

1. 在 Zeabur 开源版打开 `openlist` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**，也就是你在安装表单填的域名。
2. 用 `admin` 登录。打开**变量**，在**来自配置文件和机密变量**点 `openlist-secrets`，再点 `admin-password` 旁的眼睛图标。不用 Zeabur 开源版时，用 kubectl 读取机密变量 `openlist-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

   OpenList 每次重启都会重置一次这个密码，所以在网页里改的密码会失效。要改密码，请改机密变量再重启（见“配置”）。
3. 要用离线下载时，Aria2 令牌在同一个机密变量的 `aria2-password`。用同样的方式打开，或用 kubectl 读取：

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.aria2-password}' | base64 -d
   ```

   在 Aria2 设置里，端点填 `http://127.0.0.1:6800/jsonrpc`，令牌填这个值。

## 配置

### 更改管理员密码

在服务的**变量**标签页点 `openlist-secrets`，填入新的 `admin-password`，检查后应用。再点服务页顶部的**重启**，让 OpenList 读到新值。

### 使用离线下载

打开管理页面，进入设置，再进入其他，设好临时文件夹。接着填入“开始使用”里取得的 Aria2 端点和令牌。详见 [Offline download](https://doc.oplist.org/guide/advanced/offline-download)。

### 用密码保护文件夹

为路径新增一条 meta 记录，并设置密码。密码要覆盖子文件夹时，勾选密码后面的 Apply to sub folder。通过 WebDAV 访问时，这个密码不会生效。详见 [Meta](https://doc.oplist.org/guide/advanced/meta)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 开源版打开 `openlist` 服务，确认**概览**显示副本已就绪，再看**日志**选项卡。副本一直重启时，打开**部署记录**，切换到**事件**。如果事件显示 `OOMKilled`，到**设置** → **计算资源**调高**内存上限**。详见[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

域名或证书一直处于等待中时，看[添加公开域名与 HTTPS](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)的“遇到问题”。
