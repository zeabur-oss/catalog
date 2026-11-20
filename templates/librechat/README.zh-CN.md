## 简介

[LibreChat](https://github.com/danny-avila/LibreChat) 是 AI 模型的聊天应用。大家登录后和模型对话，也能搜索自己的对话。

这个模板在你的域名后面运行 LibreChat，用 MongoDB 存数据，Meilisearch 做搜索，并以含 pgvector 的 PostgreSQL 检索文档。聊天和文档嵌入都使用你在安装表单选的 OpenAI API 密钥。

## 开始使用

1. 等所有服务显示为运行中，再打开 `librechat` 服务的域名，例如 `https://chat.example.com`。
2. 模板在部署时用你在安装表单填的管理员邮箱创建管理员账号。获取它的密码。密码在机密 `app-secrets` 的 `admin-password` 字段：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 用管理员邮箱和这个密码登录。公开注册已关闭，其他人无法自行注册。之后再改机密，不会重置已有账号的密码。

`app-secrets` 里其他的字段（`mongo-password`、`jwt-secret`、`jwt-refresh-secret`、`creds-key`、`creds-iv` 和 `meili-key`）是 LibreChat 自己的服务使用。请在数据存在期间一直保留。

## 配置

### 更换 OpenAI API 密钥

密钥在你于安装表单选的机密的 `OPENAI_API_KEY` 字段。把新的值写进去，再重启用到它的两个部署：

```sh
kubectl -n <namespace> rollout restart deployment/librechat deployment/rag-api
```

详见 [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai)。
