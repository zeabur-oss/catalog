## 簡介

[Qdrant](https://qdrant.tech/) 是向量資料庫，有 REST API、gRPC API 和網頁管理介面。

這個範本在你的網域後面跑一個 Qdrant，資料存在磁碟區裡。每個請求都要帶 API 金鑰，金鑰在部署時自動產生。

## 開始使用

1. 在 Zeabur 打開 `qdrant` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，也就是你在安裝表單填的網域。
2. 取得 API 金鑰。打開**環境變數**，在**來自設定檔和機密變數**點 `qdrant-secrets`，再按 `api-key` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `qdrant-secrets` 的 `api-key`：

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

在服務的**環境變數**分頁點 `qdrant-secrets`，填入新的 `api-key`，檢查後套用。執行中的副本會繼續用舊值，直到你按服務頁上方的**重新啟動**。所有用到舊金鑰的用戶端都要跟著更新。
