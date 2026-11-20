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
