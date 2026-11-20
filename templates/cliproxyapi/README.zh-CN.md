## 简介

[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 是 API 代理。客户端把 OpenAI 或 Gemini 格式的请求发给它，它再转给你配置好的模型服务。

这个模板在你的域名后面运行一个 CLIProxyAPI。API 由密钥保护，密钥在部署时自动生成。它没有网页，也不用登录。管理 API 不在域名上。

## 开始使用

1. 等服务显示为运行中，记下它的域名，例如 `https://cliproxyapi.example.com`。
2. 获取 API 密钥。密钥在机密 `app-secrets` 的 `api-key` 字段：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

3. 把客户端指向 `https://cliproxyapi.example.com/v1`（Gemini 格式的客户端用 `/v1beta`），并把密钥当作 Bearer 令牌发送。没带密钥的调用会得到 `401`。
4. 需要管理密钥时再获取。它在机密 `app-secrets` 的 `management-key` 字段：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.management-key}' | base64 -d
   ```

API 密钥只管客户端接入。模型服务的凭证要另外配置，见“配置”。

## 配置

### 调用管理 API

管理 API 只接受来自 Pod 内部的调用。先用 `kubectl` 开一条隧道，再把管理密钥当作 Bearer 令牌发送：

```sh
kubectl -n <namespace> port-forward deployment/cliproxyapi 8317:8317 --address 127.0.0.1
curl -H 'Authorization: Bearer <管理密钥>' http://localhost:8317/v8/management/config
```

详见 [Management API v8](https://help.router-for.me/management/apiv8.html)。

### 添加模型服务的凭证文件

上传 CLIProxyAPI 登录命令生成的凭证文件。先开好上一节的隧道：

```sh
curl -X POST -H 'Authorization: Bearer <管理密钥>' \
  -F file=@<凭证文件>.json \
  http://localhost:8317/v8/management/credentials
```

文件存在卷上，重启后仍在。详见 [Management API v8](https://help.router-for.me/management/apiv8.html) 和 [Authentication Directory](https://help.router-for.me/configuration/auth-dir.html)。

通过 `/config` 写入的模型服务密钥只会保留到 Pod 重启，因为模板每次启动都会重新生成配置文件。

## 故障排除

### 管理 API 返回 401 或不再响应

同一个地址连续发错五次密钥，会被封禁约 30 分钟。请确认用的是 `management-key` 的值，不是 `api-key`，并等封禁结束。详见 [Management API v8](https://help.router-for.me/management/apiv8.html)。
