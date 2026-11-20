## 簡介

[Meilisearch](https://www.meilisearch.com/) 是有 REST API 的搜尋引擎。你把文件送進去，它在幾毫秒內回應搜尋請求。

這個範本在你的網域後面跑一個 Meilisearch，索引存在磁碟區上。每個請求都要帶金鑰。它沒有網頁，也不用登入。

## 開始使用

1. 在 Zeabur 打開 `meilisearch` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `search.example.com`。
2. 取得主金鑰。打開**環境變數**，在**來自設定檔和機密變數**點 `meilisearch-secrets`，再按 `master-key` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `meilisearch-secrets` 的 `master-key`：

   ```sh
   kubectl -n <namespace> get secret meilisearch-secrets -o jsonpath='{.data.master-key}' | base64 -d
   ```

3. 對這個網域送出請求，並帶上標頭 `Authorization: Bearer <金鑰>`。例如列出 API 金鑰：

   ```sh
   curl https://search.example.com/keys -H 'Authorization: Bearer <主金鑰>'
   ```

主金鑰只用來管理 API 金鑰。給用戶端的要用搜尋金鑰。

## 設定

### 取得給用戶端的金鑰

Meilisearch 會替你建立 Default Search API Key 和 Default Admin API Key，上面的 `GET /keys` 請求可以列出它們。網頁或行動應用程式用搜尋金鑰，你的伺服器用管理金鑰新增文件。詳見 [Basic security](https://www.meilisearch.com/docs/learn/security/basic_security)。

### 建立傾印檔

傾印檔（dump）是整個實例資料的備份。用有權限的 API 金鑰建立：

```sh
curl -X POST https://search.example.com/dumps -H 'Authorization: Bearer <金鑰>'
```

Meilisearch 會把傾印檔寫到磁碟區上的 `/meili_data/dumps`。詳見 [Dumps](https://www.meilisearch.com/docs/learn/data_backup/dumps)。

## 疑難排解

### 請求回傳 401

Meilisearch 會拒絕沒有有效金鑰的請求。請在 `Authorization: Bearer` 標頭帶上金鑰，並確認用的是這個實例的金鑰。詳見 [Basic security](https://www.meilisearch.com/docs/learn/security/basic_security)。
