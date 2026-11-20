## 简介

[LobeChat](https://github.com/lobehub/lobe-chat) 是 AI 模型的聊天界面。每个人的对话都存在自己的浏览器里。

这个模板在你的域名后面运行 LobeChat。页面由访问码保护，服务器使用你在安装表单选的 OpenAI API 密钥。没有用户账号。

## 开始使用

1. 等服务显示为运行中，再打开它的域名，例如 `https://chat.example.com`。
2. LobeChat 要求输入访问码时，填入访问码。访问码在机密 `lobechat-secrets` 的 `access-code` 字段：

   ```sh
   kubectl -n <namespace> get secret lobechat-secrets -o jsonpath='{.data.access-code}' | base64 -d
   ```

3. 开始聊天。对话使用你在安装表单选的机密里的 OpenAI 密钥（`api-key` 字段）。每个人的对话记录都留在自己的浏览器里。

## 配置

### 更换访问码

把新的值写进 `lobechat-secrets` 的 `access-code` 字段，再重启部署，让它读到新值：

```sh
kubectl -n <namespace> rollout restart deployment/lobechat
```

请把新的访问码告诉所有使用这个页面的人。详见 [ACCESS_CODE](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/basic.mdx)。

### 更换 OpenAI API 密钥

把新的密钥写进你在安装表单选的机密的 `api-key` 字段，再重启部署：

```sh
kubectl -n <namespace> rollout restart deployment/lobechat
```

详见 [OPENAI_API_KEY](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/model-provider.mdx)。
