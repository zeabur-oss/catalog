## 简介

[Qdrant](https://qdrant.tech/) 是向量数据库，有 REST API、gRPC API 和网页管理界面。

这个模板在你的域名后面运行一个 Qdrant，数据存在卷里。每个请求都要带 API 密钥，密钥在部署时自动生成。

## 开始使用

1. 在 Zeabur 打开 `qdrant` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**，也就是你在安装表单填的域名。
2. 获取 API 密钥。打开**变量**，在**来自配置文件和机密变量**点 `qdrant-secrets`，再点 `api-key` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `qdrant-secrets` 的 `api-key`：

   ```sh
   kubectl -n <namespace> get secret qdrant-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

3. 打开 `https://<你的域名>/dashboard`，输入 API 密钥。
4. 每个 REST 请求都要在 `api-key` 请求头带上同一把密钥：

   ```sh
   curl https://<你的域名>/collections --header 'api-key: <api-key>'
   ```

5. gRPC 端口（`6334`）不在域名后面。同一个项目里带有标签 `templates.zeabur.com/qdrant-client=true` 的 Pod，可以用 `qdrant:6333` 连 REST，用 `qdrant:6334` 连 gRPC。

## 配置

### 连接客户端

把 API 密钥交给客户端。以 Python 为例：

```python
from qdrant_client import QdrantClient

client = QdrantClient(url="https://<你的域名>:443", api_key="<api-key>")
```

详见 Qdrant 文档的 [Security](https://qdrant.tech/documentation/security/)。

### 更换 API 密钥

在服务的**变量**标签页点 `qdrant-secrets`，填入新的 `api-key`，检查后应用。运行中的副本会继续用旧值，直到你点服务页顶部的**重启**。所有用到旧密钥的客户端都要同步更新。
