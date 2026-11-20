## 简介

[Open WebUI](https://github.com/open-webui/open-webui) 是大语言模型的聊天界面。你把它连上模型服务商，大家就能在浏览器里和模型对话。

这个模板在你的域名后面运行 Open WebUI，并附带供 pipeline 使用的 `pipeline` 服务。注册功能已关闭，也还没有连接任何模型服务商。

## 开始使用

1. 等服务显示为运行中，打开它的域名，例如 `https://chat.example.com`。
2. 用你在安装表单填的管理员邮箱登录。密码在机密 `app-secrets` 的 `admin-password` 字段：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Open WebUI 在第一次启动、还没有任何用户时创建管理员账号。之后改机密不会改变密码，要改请在 Open WebUI 里改。
4. 注册功能已关闭，其他用户由管理员创建。
5. 添加模型服务商。请见“配置”。
6. 机密 `app-secrets` 里还有两个日常使用不需要的字段。`session-key` 是网页界面的密钥，`pipeline-key` 是网页界面调用 `pipeline` 服务用的 API 密钥：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.session-key}' | base64 -d
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.pipeline-key}' | base64 -d
   ```

## 配置

### 添加模型服务商

以管理员登录，打开 Settings，再到 Connections（管理员区域）。在 Manage OpenAI API Connections 旁选 Add Connection。填入服务商兼容 OpenAI 的 API 地址和 API 密钥，再保存。OpenAI 的地址是 `https://api.openai.com/v1`。详见 Open WebUI 文档的 [OpenAI](https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai)。

模板启动 Open WebUI 时，把 `ENABLE_PERSISTENT_CONFIG` 设为 `false`。这对你改的设置有什么影响，请见“故障排除”。

## 故障排除

### 在管理员设置里改的设置，重启后不见了

`ENABLE_PERSISTENT_CONFIG` 设为 `false` 时，Open WebUI 每次启动都读取环境变量里的设置。你可以在管理员设置里编辑，但改动不会被保存。重启后请再改一次。详见 Open WebUI 文档的 [Environment variable configuration](https://docs.openwebui.com/reference/env-configuration)。
