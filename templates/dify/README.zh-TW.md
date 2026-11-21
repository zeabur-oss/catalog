## 簡介

[Dify](https://github.com/langgenius/dify) 是開發 LLM 應用程式的平台。你在網頁編輯器裡設計工作流程、代理和對話應用程式，再發布成網頁應用程式或 API。

這個範本在你的網域後面跑 Dify，包含 API、背景工作程式、外掛程式服務、程式碼沙箱、PostgreSQL 和佇列。Dify 把文件和上傳的檔案存在你在安裝表單填寫的 S3 相容儲存空間。

## 開始使用

1. 在 Zeabur 開源版打開 `dify` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `dify.example.com`。Dify 的其他服務（例如 `dify-api`）也要就緒。
2. 取得初始化密碼。打開 `dify-api` 服務的**環境變數**，在**來自設定檔和機密變數**點 `dify-secrets`，再按 `init-password` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `dify-secrets` 的 `init-password`：

   ```sh
   kubectl -n <namespace> get secret dify-secrets -o jsonpath='{.data.init-password}' | base64 -d
   ```

3. 開啟 `https://dify.example.com/install`。Dify 會先要求輸入初始化密碼，別人就無法搶先佔用這個實例。輸入密碼後，建立管理員帳號。
4. 用剛建立的帳號登入 `https://dify.example.com`。

`dify-secrets` 裡其他的欄位（`app-key`、`redis-password` 和 `sandbox-key`）是 Dify 自己的服務使用，請不要外流。

## 設定

### 加入模型服務

連上模型服務之前，Dify 沒有任何模型可用。在 Dify 開啟 Integrations > Model Provider，安裝你要用的服務，點 Add API Key，輸入你的金鑰。Dify 會先檢查金鑰，通過後才讓這個服務可用。詳見 [Model Providers](https://docs.dify.ai/en/use-dify/workspace/model-providers)。

### 備份資料

Dify 把帳號、應用和設定存在 `dify-db` 資料庫，裡面有 `dify` 和 `dify_plugin` 兩個資料庫。在專案打開 `dify-db`，到**備份**設定 S3 備份，備份 `dify-db` 時兩個資料庫都包含在內。文件和上傳的檔案放在你在安裝表單填的 S3 儲存貯體，請用儲存服務自己的功能備份它。安裝的外掛放在磁碟區 `dify-plugin-storage`，掛載在 `/app/storage`。

備份時一併保管機密變數 `dify-secrets`。Dify 用 `app-key` 簽署權杖和檔案網址，並加密 OAuth 用戶端憑證。詳見 Zeabur 開源版文件的[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)和 Dify 文件的 [SECRET_KEY](https://docs.dify.ai/en/self-host/configuration/environments)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `dify` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。服務變慢或一直重新連線時，打開**用量**分頁，出現**接近記憶體上限**就用同樣的方式調高上限。`dify` 把請求轉給 `dify-api` 和 `dify-web`，所以這兩個服務和 `dify-worker` 也用同樣的方式檢查。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 開源版時，用 `kubectl -n <namespace> logs deployment/dify` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 開源版文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。
