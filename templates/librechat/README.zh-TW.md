## 簡介

[LibreChat](https://github.com/danny-avila/LibreChat) 是 AI 模型的聊天應用程式。大家登入後和模型對話，也能搜尋自己的對話。

這個範本在你的網域後面跑 LibreChat，用 MongoDB 存資料，Meilisearch 做搜尋，並以含 pgvector 的 PostgreSQL 搜尋文件。聊天和文件嵌入都使用你在安裝表單選的 OpenAI API 金鑰。

## 開始使用

1. 在 Zeabur 開源版打開 `librechat` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `chat.example.com`。
2. 範本在部署時用你在安裝表單填的管理員 Email 建立管理員帳號。取得它的密碼。打開**環境變數**，在**來自設定檔和機密變數**點 `app-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `app-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 用管理員 Email 和這組密碼登入。公開註冊已關閉，其他人無法自行註冊。之後再改機密，不會重設已有帳號的密碼。

`app-secrets` 裡其他的欄位（`mongo-password`、`jwt-secret`、`jwt-refresh-secret`、`creds-key`、`creds-iv` 和 `meili-key`）是 LibreChat 自己的服務使用。請在資料存在的期間一直保留。

## 設定

### 更換 OpenAI API 金鑰

金鑰在你於安裝表單選的機密的 `OPENAI_API_KEY`。在 `librechat` 服務的**環境變數**分頁，於**來自設定檔和機密變數**點這個機密，填入新的值，檢查後套用。執行中的副本會繼續用舊金鑰，所以要分別按 `librechat` 和 `rag-api` 服務頁上方的**重新啟動**。

詳見 [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai)。

### 備份資料

LibreChat 把使用者和對話存在 `mongodb` 資料庫，文件的嵌入向量存在 `rag-db` 資料庫。在專案分別打開這兩個資料庫，到**備份**設定 S3 備份。想直接拿到檔案，按**更多** → **匯出**。使用者上傳的檔案放在磁碟區 `librechat-data`，掛載在 `/app/uploads`。用 `librechat` 服務的**檔案**分頁複製。資料夾很大時，改用 kubectl。`kubectl -n <namespace> get pods` 會列出副本名稱：

```sh
kubectl -n <namespace> cp <replica>:/app/uploads ./librechat-uploads
```

如開始使用所說，備份時一併保管機密變數 `app-secrets`。詳見 Zeabur 開源版文件的[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)和[下載資料庫傾印檔](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#下載資料庫傾印檔)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `librechat` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。服務變慢或一直重新連線時，打開**用量**分頁，出現**接近記憶體上限**就用同樣的方式調高上限。聊天正常但檔案搜尋失敗時，用同樣的方式檢查 `rag-api` 服務。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 開源版時，用 `kubectl -n <namespace> logs deployment/librechat` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 開源版文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### 對話或檔案搜尋失敗，出現模型服務商的錯誤

對話和文件嵌入都用你在安裝表單選的機密變數裡的 `OPENAI_API_KEY`。請求失敗時，先看 `librechat` 服務的**日誌**分頁。檔案搜尋失敗時，也看 `rag-api` 服務。再確認三件事。金鑰要有效。你選的模型要在這把金鑰的可用範圍內。你的帳號在 OpenAI 也不能超過頻率限制或額度。詳見 LibreChat 文件的 [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai)。
