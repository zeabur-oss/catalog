## 简介

[OpenClaw](https://github.com/openclaw/openclaw) 是个人 AI 助手。Gateway（网关）负责运行助手，网页控制界面（Control UI）让你和它对话并管理它。它也能连接 WhatsApp、Telegram、Slack、Discord 等通讯软件。

这个模板在你的域名后面运行一个 Gateway，使用你在安装表单填的、兼容 OpenAI 的模型服务商。

## 开始使用

1. 等服务显示为运行中，打开它的域名，例如 `https://openclaw.example.com`。
2. 登录界面有一个 Gateway secret 字段，把网关令牌粘贴进去。令牌在机密 `openclaw-secrets` 的 `gateway-token` 字段：

   ```sh
   kubectl -n <namespace> get secret openclaw-secrets -o jsonpath='{.data.gateway-token}' | base64 -d
   ```

3. 新的浏览器在你放行一次之前，会显示 `disconnected (1008): pairing required`。请保持页面打开，列出请求，再放行你的那一条。放行后页面会自己连接：

   ```sh
   kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices list
   kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices approve <requestId>
   ```

4. 模型服务商就是你在安装表单填的 base URL、模型和 API 密钥。API 密钥在你于表单里指定的机密，字段是 `api-key`，不在 `openclaw-secrets` 里。

## 配置

### 更换服务商的 API 密钥

把新的密钥写进你在安装表单里指定的机密的 `api-key` 字段，再重启 Deployment，让它读到新值：

```sh
kubectl -n <namespace> rollout restart deployment/openclaw
```

base URL 和模型只在第一次启动时写进 Gateway 的配置文件。之后再改安装表单，不会改变它们。

### 移除已配对的浏览器

列出已配对的设备，再移除不用的那一个：

```sh
kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices list
kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices remove <deviceId>
```

详见 OpenClaw 文档的 [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair)。

## 故障排除

### 另一个浏览器要求配对

每个浏览器配置文件各自配对，一个配置文件不能沿用另一个的放行。请对每个配置文件用 `devices approve` 放行一次，并用 `devices remove` 移除旧的条目。详见 OpenClaw 文档的 [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair)。
