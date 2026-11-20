## 簡介

[Qdrant](https://qdrant.tech/) 是向量資料庫，有 REST API、gRPC API 和網頁管理介面。

這個範本在你的網域後面跑一個 Qdrant，資料存在磁碟區裡。每個請求都要帶 API 金鑰，金鑰在部署時自動產生。

## 開始使用

1. 等服務顯示為執行中，記下它的網域，例如 `https://qdrant.example.com`。
2. 取得 API 金鑰。金鑰在機密 `qdrant-secrets` 的 `api-key` 欄位：

   ```sh
   kubectl -n <namespace> get secret qdrant-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

3. 開啟 `https://<你的網域>/dashboard`，輸入 API 金鑰。
4. 每個 REST 請求都要在 `api-key` 標頭帶上同一把金鑰：

   ```sh
   curl https://<你的網域>/collections --header 'api-key: <api-key>'
   ```

5. gRPC 連接埠（`6334`）不在網域後面。同一個專案裡帶有標籤 `templates.zeabur.com/qdrant-client=true` 的 Pod，可以用 `qdrant:6333` 連 REST，用 `qdrant:6334` 連 gRPC。

## 設定

### 連接用戶端

把 API 金鑰交給用戶端。以 Python 為例：

```python
from qdrant_client import QdrantClient

client = QdrantClient(url="https://<你的網域>:443", api_key="<api-key>")
```

詳見 Qdrant 文件的 [Security](https://qdrant.tech/documentation/security/)。

### 更換 API 金鑰

把新的值寫進 `qdrant-secrets` 的 `api-key` 欄位，再重新啟動 StatefulSet，讓它讀到新值：

```sh
kubectl -n <namespace> rollout restart statefulset/qdrant
```

所有用到舊金鑰的用戶端都要跟著更新。
