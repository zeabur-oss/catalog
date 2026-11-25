## 簡介

[LiteLLM](https://github.com/BerriAI/litellm) 是代理，在多家 LLM 服務前面提供一個相容 OpenAI 的 API。它也有管理頁面，用來管理模型、金鑰和花費。

這個範本在你的網域後面跑 LiteLLM，模型、金鑰和花費都存在 PostgreSQL。

## 開始使用

1. 在 Zeabur 開源版打開 `litellm` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `litellm.example.com`。管理頁面在這個網域的 `/ui`。
2. 取得主金鑰。打開**環境變數**，在**來自設定檔和機密變數**點 `litellm-secrets`，再按 `master-key` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `litellm-secrets` 的 `master-key`：

   ```sh
   kubectl -n <namespace> get secret litellm-secrets -o jsonpath='{.data.master-key}' | base64 -d
   ```

   實際使用的金鑰是 `sk-` 加上這個值。
3. 用 `admin` 登入，密碼是 `sk-` 加上剛才讀到的值。
4. 同一把 `sk-` 金鑰也是呼叫 `https://litellm.example.com` 的管理員 API 金鑰。模型和給用戶端的金鑰都在管理頁面裡新增。

`litellm-secrets` 的 `salt-key` 用來加密資料庫裡模型服務的認證資訊。加入模型後，請不要再更改它。

## 設定

### 新增模型

在管理頁面開啟 Models + Endpoints，選擇模型服務並填入它的 API 金鑰。在頁面新增的模型存在資料庫裡，不用編輯檔案，也不用重新啟動。詳見 [Model Management](https://docs.litellm.ai/docs/proxy/model_management)。

### 為用戶端建立金鑰

請不要把主金鑰交給應用程式。在管理頁面為每個用戶端建立虛擬金鑰，再把用戶端指向 `https://litellm.example.com/v1` 並使用這把金鑰。詳見 [Virtual Keys](https://docs.litellm.ai/docs/proxy/virtual_keys) 和 [Langchain, OpenAI SDK 等用戶端](https://docs.litellm.ai/docs/proxy/user_keys)。

### 備份資料

LiteLLM 把模型、金鑰和花費存在 `litellm-db` 資料庫，沒有放任何東西在磁碟區。在專案打開 `litellm-db`，到**備份**設定 S3 備份。想直接拿到檔案，按**更多** → **匯出**。備份時一併保管機密變數 `litellm-secrets`，因為資料庫裡的模型服務憑證要靠 `salt-key` 解密。詳見 Zeabur 開源版文件的[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)和[下載資料庫傾印檔](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#下載資料庫傾印檔)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `litellm` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 開源版時，用 `kubectl -n <namespace> logs deployment/litellm` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 開源版文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### 請求失敗，出現模型服務商的錯誤

先看 `litellm` 服務的**日誌**分頁。接著在管理頁面打開 Models + Endpoints，檢查出錯的那個模型。模型名稱要是你的服務商接受的名稱。這個模型的 API 金鑰和端點要有效。你的帳號在服務商那裡也不能超過頻率限制或額度。詳見 LiteLLM 文件的 [AI Providers](https://docs.litellm.ai/docs/providers)。
