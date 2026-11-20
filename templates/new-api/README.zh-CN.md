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

## 故障排除

### 用自动生成的密码登录失败

密码不只是机密的值，它是以 `Aa1-` 开头的。请用“开始使用”里的命令打印出完整的密码。
