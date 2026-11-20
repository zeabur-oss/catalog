## 简介

[Meilisearch](https://www.meilisearch.com/) 是有 REST API 的搜索引擎。你把文档发给它，它在几毫秒内响应搜索请求。

这个模板在你的域名后面运行一个 Meilisearch，索引存在卷上。每个请求都要带密钥。它没有网页，也不用登录。

## 开始使用

1. 等服务显示为运行中，记下它的域名，例如 `https://search.example.com`。
2. 获取主密钥。密钥在机密 `meilisearch-secrets` 的 `master-key` 字段：

   ```sh
   kubectl -n <namespace> get secret meilisearch-secrets -o jsonpath='{.data.master-key}' | base64 -d
   ```

3. 对这个域名发送请求，并带上请求头 `Authorization: Bearer <密钥>`。例如列出 API 密钥：

   ```sh
   curl https://search.example.com/keys -H 'Authorization: Bearer <主密钥>'
   ```

主密钥只用来管理 API 密钥。给客户端的要用搜索密钥。

## 配置

### 获取给客户端的密钥

Meilisearch 会替你创建 Default Search API Key 和 Default Admin API Key，上面的 `GET /keys` 请求可以列出它们。网页或移动应用用搜索密钥，你的服务器用管理密钥添加文档。详见 [Basic security](https://www.meilisearch.com/docs/learn/security/basic_security)。

### 创建转储文件

转储文件（dump）是整个实例数据的备份。用有权限的 API 密钥创建：

```sh
curl -X POST https://search.example.com/dumps -H 'Authorization: Bearer <密钥>'
```

Meilisearch 会把转储文件写到卷上的 `/meili_data/dumps`。详见 [Dumps](https://www.meilisearch.com/docs/learn/data_backup/dumps)。

## 故障排除

### 请求返回 401

Meilisearch 会拒绝没有有效密钥的请求。请在 `Authorization: Bearer` 请求头带上密钥，并确认用的是这个实例的密钥。详见 [Basic security](https://www.meilisearch.com/docs/learn/security/basic_security)。
