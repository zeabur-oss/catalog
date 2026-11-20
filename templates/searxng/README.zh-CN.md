## 简介

[SearXNG](https://github.com/searxng/searxng) 是元搜索引擎。它把你的查询发送到许多搜索引擎，再把结果合并成一份列表，不需要账号，SearXNG 本身也不追踪你。

这个模板在你的域名后面运行一个 SearXNG。HTML 和 JSON 两种结果都已开启。

## 开始使用

1. 等服务显示为运行中，打开它的域名，例如 `https://searxng.example.com`。
2. 直接搜索。这里没有账号，也不用登录，知道网址的人都能使用。
3. 服务器的签名密钥在机密 `app-secrets` 的 `signing-key` 字段。搜索时用不到：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.signing-key}' | base64 -d
   ```

## 配置

### 获取 JSON 结果

在搜索请求加上 `format=json`：

```sh
curl 'https://<你的域名>/search?q=searxng&format=json'
```

详见 [Search API](https://docs.searxng.org/dev/search_api.html)。

## 故障排除

### 使用 format=csv 或 format=rss 的请求得到 403

这个模板只开启 `html` 和 `json` 两种格式。没开启的格式会得到 `403 Forbidden`。详见 [Search API](https://docs.searxng.org/dev/search_api.html)。
