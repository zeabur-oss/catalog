## 简介

[OpenClaw](https://github.com/openclaw/openclaw) 是个人 AI 助手。Gateway（网关）负责运行助手，网页控制界面（Control UI）让你和它对话并管理它。它也能连接 WhatsApp、Telegram、Slack、Discord 等通讯软件。

这个模板在你的域名后面运行一个 Gateway，使用你在安装表单填的、兼容 OpenAI 的模型服务商。

## 开始使用

1. 在 Zeabur 打开 `openclaw` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，就是你在安装表单填的域名，例如 `openclaw.example.com`。
2. 登录界面有一个 Gateway secret 字段，把网关令牌粘贴进去。令牌从 `openclaw` 服务获取：打开**变量**，在**来自配置文件和机密变量**中点击 `openclaw-secrets`，再点击 `gateway-token` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `openclaw-secrets` 的 `gateway-token`：

   ```sh
   kubectl -n <namespace> get secret openclaw-secrets -o jsonpath='{.data.gateway-token}' | base64 -d
   ```

3. 新的浏览器在你放行一次之前，会显示 `disconnected (1008): pairing required`。请保持页面打开。点击 `openclaw` 服务页面顶部的**终端**，列出请求，再放行你的那一条。放行后页面会自己连接：

   ```sh
   node /app/openclaw.mjs devices list
   node /app/openclaw.mjs devices approve <requestId>
   ```

4. 模型服务商就是你在安装表单填的 base URL、模型和 API 密钥。API 密钥在你于表单里指定的机密变量，字段是 `api-key`，不在 `openclaw-secrets` 里。要查看，打开 `openclaw` 服务的**变量**，点击那个机密变量。

## 配置

### 更换服务商的 API 密钥

在 `openclaw` 服务打开**变量**，点击你在安装表单指定的机密变量。填入新的 `api-key`，检查后应用，再点击服务页面顶部的**重启**，让 Gateway 读到新值。

base URL 和模型只在第一次启动时写进 Gateway 的配置文件。之后再改安装表单，不会改变它们。

### 移除已配对的浏览器

点击 `openclaw` 服务页面顶部的**终端**。列出已配对的设备，再移除不用的那一个：

```sh
node /app/openclaw.mjs devices list
node /app/openclaw.mjs devices remove <deviceId>
```

详见 OpenClaw 文档的 [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `openclaw` 服务，确认**概览**显示副本已就绪，再看**日志**选项卡。副本一直重启时，打开**部署记录**，切换到**事件**。如果事件显示 `OOMKilled`，到**设置** → **计算资源**调高**内存上限**。详见[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

服务变慢或一直重新连接时，也按这些步骤检查。

域名或证书一直处于等待中时，看[添加公开域名与 HTTPS](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)的“遇到问题”。

### 另一个浏览器要求配对

每个浏览器配置文件各自配对，一个配置文件不能沿用另一个的放行。请在**终端**对每个配置文件用 `devices approve` 放行一次，并用 `devices remove` 移除旧的条目。详见 OpenClaw 文档的 [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair)。

### 显示 All models failed 或模型服务返回错误

OpenClaw 用安装表单填写的 API 地址和模型发送模型请求。依次检查：

- 模型名称在你的模型服务上确实存在。名称填错很常见。
- 你在安装表单指定的机密变量里，键 `api-key` 的 API 密钥有效，也没有超过模型服务的速率限制或额度。
- API 地址是 OpenAI 兼容 API 的 HTTPS 地址。

打开 `openclaw` 服务的**日志**选项卡，查看模型服务返回的错误。API 地址和模型只在第一次启动时从安装表单读取。之后要改，用控制界面的 **Config** 选项卡。API 地址是 `models.providers.custom.baseUrl`，模型名称要在 `models.providers.custom.models` 和 `agents.defaults.model.primary` 两处都改。详见 OpenClaw 文档的 [Model providers](https://docs.openclaw.ai/concepts/model-providers) 和 [Configuration](https://docs.openclaw.ai/gateway/configuration)。
