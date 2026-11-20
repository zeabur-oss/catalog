## 简介

[Browserless](https://github.com/browserless/browserless) 把无头 Chrome 变成一个服务。Puppeteer、Playwright 等自动化工具通过 WebSocket 或 HTTP 连上它，自己的机器上就不用装浏览器。

这个模板在你的域名后面运行一个 Browserless。每次调用都要带令牌，令牌在部署时自动生成。

## 开始使用

1. 等服务显示为运行中，记下它的域名，例如 `https://browserless.example.com`。
2. 获取令牌。令牌在机密 `browserless-secrets` 的 `token` 字段：

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

把新的值写进 `browserless-secrets` 的 `token` 字段，再重启部署，让它读到新值：

```sh
kubectl -n <namespace> rollout restart deployment/browserless
```

所有用到旧令牌的客户端都要同步更新。

## 故障排除

### 会话在排队或超时

模板限制同时打开的浏览器数量（`CONCURRENT`），其余的会排队（`QUEUED`）。先关掉客户端不再需要的浏览器；要调高这两个值之前，先调高服务的内存上限。详见 [Docker configuration](https://docs.browserless.io/baas/docker/config)。
