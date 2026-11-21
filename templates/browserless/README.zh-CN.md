## 简介

[Browserless](https://github.com/browserless/browserless) 把无头 Chrome 变成一个服务。Puppeteer、Playwright 等自动化工具通过 WebSocket 或 HTTP 连上它，自己的机器上就不用装浏览器。

这个模板在你的域名后面运行一个 Browserless。每次调用都要带令牌，令牌在部署时自动生成。

## 开始使用

1. 在 Zeabur 开源版打开 `browserless` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `browserless.example.com`。
2. 获取令牌。打开**变量**，在**来自配置文件和机密变量**中点击 `browserless-secrets`，再点击 `token` 旁的眼睛图标。不用 Zeabur 开源版时，用 kubectl 读取机密变量 `browserless-secrets` 的 `token`：

   ```sh
   kubectl -n <namespace> get secret browserless-secrets -o jsonpath='{.data.token}' | base64 -d
   ```

3. 把客户端指向这个域名，并在 `Authorization: Bearer <token>` 请求头带上令牌。客户端不能设置请求头时，在 URL 加上 `?token=<token>`。以 Puppeteer 为例：

   ```js
   const browser = await puppeteer.connect({
     browserWSEndpoint: "wss://browserless.example.com?token=<token>",
   });
   ```

   这个服务没有网页，也不用登录。没带令牌的调用会得到 `401`。

## 配置

### 从 Playwright 连接

使用 `chromium.connectOverCDP("wss://<你的域名>?token=<token>")`。详见 Browserless 文档的 [Playwright connection](https://docs.browserless.io/examples/playwright-connection)。

### 更换令牌

在服务的**变量**标签页点击 `browserless-secrets`，填入新的 `token`，检查后应用。运行中的副本会继续使用旧令牌，直到你点击服务页面顶部的**重启**。所有用到旧令牌的客户端都要同步更新。

## 故障排除

### 会话在排队或超时

模板限制同时打开的浏览器数量（`CONCURRENT`），其余的会排队（`QUEUED`）。先关掉客户端不再需要的浏览器；要调高这两个值之前，先调高服务的内存上限。详见 [Docker configuration](https://docs.browserless.io/baas/docker/config)。

### 页面显示 502 或打不开

在 Zeabur 开源版打开 `browserless` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。服务变慢或一直重新连接时，打开**用量**标签页，出现**接近内存上限**就用同样的方式调高上限。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

不用 Zeabur 开源版时，用 `kubectl -n <namespace> logs deployment/browserless` 查看日志，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 查看事件。

详见 Zeabur 开源版文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。
