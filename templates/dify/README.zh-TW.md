## 簡介

[Dify](https://github.com/langgenius/dify) 是開發 LLM 應用程式的平台。你在網頁編輯器裡設計工作流程、代理和對話應用程式，再發布成網頁應用程式或 API。

這個範本在你的網域後面跑 Dify，包含 API、背景工作程式、外掛程式服務、程式碼沙箱、PostgreSQL 和佇列。Dify 把文件和上傳的檔案存在你在安裝表單填寫的 S3 相容儲存空間。

## 開始使用

1. 在 Zeabur 打開 `dify` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `dify.example.com`。Dify 的其他服務（例如 `dify-api`）也要就緒。
2. 取得初始化密碼。打開 `dify-api` 服務的**環境變數**，在**來自設定檔和機密變數**點 `dify-secrets`，再按 `init-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `dify-secrets` 的 `init-password`：

   ```sh
   kubectl -n <namespace> get secret dify-secrets -o jsonpath='{.data.init-password}' | base64 -d
   ```

3. 開啟 `https://dify.example.com/install`。Dify 會先要求輸入初始化密碼，別人就無法搶先佔用這個實例。輸入密碼後，建立管理員帳號。
4. 用剛建立的帳號登入 `https://dify.example.com`。

`dify-secrets` 裡其他的欄位（`app-key`、`redis-password` 和 `sandbox-key`）是 Dify 自己的服務使用，請不要外流。

## 設定

### 加入模型服務

連上模型服務之前，Dify 沒有任何模型可用。在 Dify 開啟 Integrations > Model Provider，安裝你要用的服務，點 Add API Key，輸入你的金鑰。Dify 會先檢查金鑰，通過後才讓這個服務可用。詳見 [Model Providers](https://docs.dify.ai/en/use-dify/workspace/model-providers)。
