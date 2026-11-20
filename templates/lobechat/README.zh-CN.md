## 简介

[LobeChat](https://github.com/lobehub/lobe-chat) 是 AI 模型的聊天界面。每个人的对话都存在自己的浏览器里。

这个模板在你的域名后面运行 LobeChat。页面由访问码保护，服务器使用你在安装表单选的 OpenAI API 密钥。没有用户账号。

## 开始使用

1. 在 Zeabur 打开 `lobechat` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `chat.example.com`。
2. LobeChat 要求输入访问码时，填入访问码。要读取它，打开**变量**，在**来自配置文件和机密变量**中点击 `lobechat-secrets`，再点击 `access-code` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `lobechat-secrets` 的 `access-code`：

   ```sh
   kubectl -n <namespace> get secret lobechat-secrets -o jsonpath='{.data.access-code}' | base64 -d
   ```

3. 开始聊天。对话使用你在安装表单选的机密里的 OpenAI 密钥（`api-key` 字段）。每个人的对话记录都留在自己的浏览器里。

## 配置

### 更换访问码

在服务的**变量**标签页点击 `lobechat-secrets`，填入新的 `access-code`，检查后应用。运行中的副本会继续使用旧的访问码，直到你点击服务页面顶部的**重启**。请把新的访问码告诉所有使用这个页面的人。详见 [ACCESS_CODE](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/basic.mdx)。

### 更换 OpenAI API 密钥

在服务的**变量**标签页点击你在安装表单选的机密，填入新的 `api-key`，检查后应用，再点击服务页面顶部的**重启**，让副本读到新值。

详见 [OPENAI_API_KEY](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/model-provider.mdx)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `lobechat` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

不用 Zeabur 时，用 `kubectl -n <namespace> logs deployment/lobechat` 查看日志，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 查看事件。

详见 Zeabur 文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

### 对话失败，出现模型服务商的错误

服务器用你在安装表单选的机密变量里的 `api-key`，把对话发送到 OpenAI。对话失败时，先看 `lobechat` 服务的**日志**标签页，再确认三件事。密钥要有效。你选的模型要在这把密钥的可用范围内。你的账号在 OpenAI 也不能超过频率限制或额度。详见 LobeChat 文档的 [OPENAI_API_KEY](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/model-provider.mdx)。
