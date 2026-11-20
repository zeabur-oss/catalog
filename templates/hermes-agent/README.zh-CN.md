## 简介

[Hermes Agent](https://github.com/NousResearch/hermes-agent) 是通过 API 和网页管理面板运行的 AI 智能体。API 采用 OpenAI 格式，所以兼容 OpenAI 的客户端都能把这个智能体当后端。

这个模板用两个域名运行 Hermes Agent，一个给 API，一个给管理面板。智能体通过 OpenRouter，或你在安装表单填的 OpenAI 兼容端点调用模型。

## 开始使用

1. 在 Zeabur 打开 `hermes-agent` 服务。**概览**显示副本已就绪，服务就启动好了。两个域名都在**网络**标签页的**公开域名**中，例如 API 用 `hermes.example.com`，管理面板用 `hermes-admin.example.com`。
2. 获取管理面板密码。打开**变量**，在**来自配置文件和机密变量**中点击 `hermes-secrets`，再点击 `dashboard-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `hermes-secrets` 的 `dashboard-password`：

   ```sh
   kubectl -n <namespace> get secret hermes-secrets -o jsonpath='{.data.dashboard-password}' | base64 -d
   ```

3. 打开管理面板的域名，用 `admin` 和这个密码登录。
4. 获取 API 密钥。在同一个 `hermes-secrets` 列表点击 `api-key` 旁的眼睛图标。用 kubectl 的做法：

   ```sh
   kubectl -n <namespace> get secret hermes-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

5. 在 `https://hermes.example.com/v1` 调用 API，并把密钥当作 Bearer 令牌发送：

   ```sh
   curl https://hermes.example.com/v1/chat/completions \
     -H "Authorization: Bearer <API 密钥>" \
     -H "Content-Type: application/json" \
     -d '{"model": "hermes-agent", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

模型服务的 API 密钥不在 `hermes-secrets`，而是在你于安装表单选的机密的 `api-key` 字段。`hermes-secrets` 的 `dashboard-session-key` 用来签署管理面板的会话。

## 配置

### 连接客户端

Open WebUI、LobeChat、LibreChat 等兼容 OpenAI 的客户端都能使用这个智能体。把 API 基础 URL 设为 `https://hermes.example.com/v1`，API 密钥填 `hermes-secrets` 里 `api-key` 的值。详见 [API Server](https://hermes-agent.nousresearch.com/docs/user-guide/features/api-server)。

### 更换模型服务的 API 密钥

在服务的**变量**标签页，于**来自配置文件和机密变量**中点击你在安装表单选的机密，填入新的 `api-key`，检查后应用。运行中的副本会继续使用旧密钥，直到你点击服务页面顶部的**重启**。

模板每次启动都会把安装表单填的模型、端点和密钥写进 Hermes 的配置。在管理面板修改模型，只会保持到下次重启。

管理面板能管理哪些项目，详见 [Web Dashboard](https://hermes-agent.nousresearch.com/docs/user-guide/features/web-dashboard)。
