## 简介

[New API](https://github.com/QuantumNous/new-api) 是 AI 模型的网关。它给你的应用一组兼容 OpenAI 的 API，并把每个请求发到你配置好的模型服务商。

这个模板在你的域名后面运行一个 New API，并附带专属的 PostgreSQL 数据库。初始设置已经替你完成，所以不会出现设置页面，名为 `admin` 的管理员账号也已建好。

## 开始使用

1. 在 Zeabur 打开 `new-api` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，就是你在安装表单填的域名，例如 `newapi.example.com`。
2. 以 `admin` 登录。密码是 `Aa1-` 加上 `admin-password` 的值。值从 `new-api` 服务获取：打开**变量**，在**来自配置文件和机密变量**中点击 `newapi-secrets`，再点击 `admin-password` 旁的眼睛图标，并在前面加上 `Aa1-`。不用 Zeabur 时，下面的 kubectl 命令会打印出完整的密码：

   ```sh
   echo "Aa1-$(kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.admin-password}' | base64 -d)"
   ```

3. 把你的模型服务商添加为渠道（channel）。详见“配置”。
4. 机密 `newapi-secrets` 里还有两个日常使用不需要的字段。`session-secret` 是会话机密，`crypto-secret` 是缓存键用的 HMAC 机密。获取方式同上。不用 Zeabur 时，用 kubectl：

   ```sh
   kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.session-secret}' | base64 -d
   kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.crypto-secret}' | base64 -d
   ```

## 配置

### 添加模型服务商

渠道（channel）是对一个模型服务商的一条连接。以 `admin` 登录后，打开左侧菜单的 Channels，选 Add Channel。选择服务商类型，填入名称和服务商的 API 密钥，并勾选这个渠道支持的模型。用渠道上的 Test 确认能正常工作。详见 New API 文档的 [Channel Management](https://docs.newapi.ai/en/docs/guide/feature-guide/admin/channel)。

### 从应用调用网关

在控制台创建 API 令牌，详见 [API Tokens](https://docs.newapi.ai/en/docs/guide/console/api-token)。接着把应用指向 `https://<你的域名>/v1`，并把令牌当作 Bearer 令牌发送：

```sh
curl https://newapi.example.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-<token>" \
  -d '{"model":"<model>","messages":[{"role":"user","content":"Hello"}]}'
```

详见 New API 文档的 [Using API and Playground](https://docs.newapi.ai/en/docs/guide/feature-guide/user/api)。

### 更改管理员密码

New API 只在第一次启动、创建管理员时读取 `admin-password`。之后改机密变量不会改变密码。请登录 New API 后在里面更改。

### 备份数据

New API 把保存的东西都放在 `newapi-db` 数据库。模板只把日志文件写到 `/data`，那是临时空间，替换副本时会被删除，所以没有卷要备份。在项目中打开 `newapi-db` 数据库。到**备份**配置 S3 备份。想直接拿到文件，点击**更多** → **导出**。详见 Zeabur 文档的[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和[下载数据库转储文件](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#下载数据库转储文件)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `new-api` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

详见 Zeabur 文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

### 用自动生成的密码登录失败

密码不只是机密的值，它是以 `Aa1-` 开头的。请用“开始使用”里的命令打印出完整的密码。

### 请求出现服务商的错误

New API 会把服务商的错误传回来。以 `admin` 打开 Channels，检查三件事。请求里的模型名称必须是渠道列出的模型，也必须是服务商有的模型。渠道的 API 密钥必须有效，可用渠道上的 Test 确认。服务商的速率限制或额度不能已经用完。接着在 Zeabur 打开 `new-api` 服务的**日志**标签页，读错误信息。详见 New API 文档的 [Channel Management](https://docs.newapi.ai/en/docs/guide/feature-guide/admin/channel)。
