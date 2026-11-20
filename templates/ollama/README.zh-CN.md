## 简介

[Ollama](https://github.com/ollama/ollama) 运行大语言模型，并通过 HTTP API 提供服务。

这个模板用 CPU 运行一个 Ollama，并准备了 50 GiB 的卷放模型。它没有域名、没有网页，也不用登录。只有同一个命名空间里你允许的 Pod 能访问它的 API。不会预先加载任何模型。

## 开始使用

1. 其他工作负载用 Service `ollama` 的 `11434` 端口连接 Ollama，例如 `http://ollama:11434`。模板只允许同一个命名空间里、同时带有下面两个标签的 Pod：

   ```yaml
   templates.zeabur.com/template: ollama
   templates.zeabur.com/ollama-client: "true"
   ```

2. 要从你的电脑使用 API，先转发端口，再用 `http://localhost:11434`：

   ```sh
   kubectl -n <namespace> port-forward service/ollama 11434
   ```

3. 下载模型。请先看过模型的许可和大小。Pod 的内存上限是 8 GiB，所以放不下大模型：

   ```sh
   kubectl -n <namespace> exec deployment/ollama -- ollama pull <模型>
   ```

这个服务没有密码，所以模板不会创建机密。

## 配置

### 连接 OpenAI 客户端

Ollama 有兼容 OpenAI 的 API。把客户端的 base URL 设成 `http://ollama:11434/v1`。客户端需要一个 API 密钥的值，但 Ollama 会忽略它，所以填任何文字都可以。详见 Ollama 文档的 [OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility)。

### 列出和移除模型

```sh
kubectl -n <namespace> exec deployment/ollama -- ollama ls
kubectl -n <namespace> exec deployment/ollama -- ollama rm <模型>
```

详见 Ollama 文档的 [CLI reference](https://docs.ollama.com/cli)。

## 故障排除

### Pod 连接超时

Pod 需要“开始使用”里的两个标签。请检查连不上的 Pod 的标签，并在该工作负载的 Pod 模板补上缺少的标签。

### 模型很慢或加载不了

这个模板用 CPU 运行，内存上限是 8 GiB，一次只加载一个模型。运行 `ollama ps` 查看加载了什么，模型放不下时请改用较小的模型。原因可以看日志：

```sh
kubectl -n <namespace> exec deployment/ollama -- ollama ps
kubectl -n <namespace> logs deployment/ollama
```

详见 Ollama 文档的 [FAQ](https://docs.ollama.com/faq) 和 [Troubleshooting](https://docs.ollama.com/troubleshooting)。
