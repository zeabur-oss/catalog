## 简介

[LibreChat](https://github.com/danny-avila/LibreChat) 是 AI 模型的聊天应用。大家登录后和模型对话，也能搜索自己的对话。

这个模板在你的域名后面运行 LibreChat，用 MongoDB 存数据，Meilisearch 做搜索，并以含 pgvector 的 PostgreSQL 检索文档。聊天和文档嵌入都使用你在安装表单选的 OpenAI API 密钥。

## 开始使用

1. 在 Zeabur 打开 `librechat` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `chat.example.com`。
2. 模板在部署时用你在安装表单填的管理员邮箱创建管理员账号。获取它的密码。打开**变量**，在**来自配置文件和机密变量**中点击 `app-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `app-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 用管理员邮箱和这个密码登录。公开注册已关闭，其他人无法自行注册。之后再改机密，不会重置已有账号的密码。

`app-secrets` 里其他的字段（`mongo-password`、`jwt-secret`、`jwt-refresh-secret`、`creds-key`、`creds-iv` 和 `meili-key`）是 LibreChat 自己的服务使用。请在数据存在期间一直保留。

## 配置

### 更换 OpenAI API 密钥

密钥在你于安装表单选的机密的 `OPENAI_API_KEY`。在 `librechat` 服务的**变量**标签页，于**来自配置文件和机密变量**中点击这个机密，填入新的值，检查后应用。运行中的副本会继续使用旧密钥，所以要分别点击 `librechat` 和 `rag-api` 服务页面顶部的**重启**。

详见 [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai)。

### 备份数据

LibreChat 把用户和对话存在 `mongodb` 数据库，文档的嵌入向量存在 `rag-db` 数据库。在项目中分别打开这两个数据库，到**备份**配置 S3 备份。想直接拿到文件，点击**更多** → **导出**。用户上传的文件放在卷 `librechat-data` 上，挂载在 `/app/uploads`。用 `librechat` 服务的**文件**标签页复制。文件夹很大时，改用 kubectl。`kubectl -n <namespace> get pods` 会列出副本名称：

```sh
kubectl -n <namespace> cp <replica>:/app/uploads ./librechat-uploads
```

如开始使用所说，备份时一并保管机密变量 `app-secrets`。详见 Zeabur 文档的[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和[下载数据库转储文件](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#下载数据库转储文件)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `librechat` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。服务变慢或一直重新连接时，打开**用量**标签页，出现**接近内存上限**就用同样的方式调高上限。聊天正常但文件搜索失败时，用同样的方式检查 `rag-api` 服务。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

不用 Zeabur 时，用 `kubectl -n <namespace> logs deployment/librechat` 查看日志，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 查看事件。

详见 Zeabur 文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

### 对话或文件搜索失败，出现模型服务商的错误

对话和文档嵌入都用你在安装表单选的机密变量里的 `OPENAI_API_KEY`。请求失败时，先看 `librechat` 服务的**日志**标签页。文件搜索失败时，也看 `rag-api` 服务。再确认三件事。密钥要有效。你选的模型要在这把密钥的可用范围内。你的账号在 OpenAI 也不能超过频率限制或额度。详见 LibreChat 文档的 [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai)。
