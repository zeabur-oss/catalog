## 簡介

[LobeChat](https://github.com/lobehub/lobe-chat) 是 AI 模型的聊天介面。每個人的對話都存在自己的瀏覽器裡。

這個範本在你的網域後面跑 LobeChat。頁面由存取碼保護，伺服器使用你在安裝表單選的 OpenAI API 金鑰。沒有使用者帳號。

## 開始使用

1. 在 Zeabur 打開 `lobechat` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `chat.example.com`。
2. LobeChat 要求輸入存取碼時，填入存取碼。要讀取它，打開**環境變數**，在**來自設定檔和機密變數**點 `lobechat-secrets`，再按 `access-code` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `lobechat-secrets` 的 `access-code`：

   ```sh
   kubectl -n <namespace> get secret lobechat-secrets -o jsonpath='{.data.access-code}' | base64 -d
   ```

3. 開始聊天。對話使用你在安裝表單選的機密裡的 OpenAI 金鑰（`api-key` 欄位）。每個人的對話紀錄都留在自己的瀏覽器裡。

## 設定

### 更換存取碼

在服務的**環境變數**分頁點 `lobechat-secrets`，填入新的 `access-code`，檢查後套用。執行中的副本會繼續用舊的存取碼，直到你按服務頁上方的**重新啟動**。請把新的存取碼告訴所有使用這個頁面的人。詳見 [ACCESS_CODE](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/basic.mdx)。

### 更換 OpenAI API 金鑰

在服務的**環境變數**分頁點你在安裝表單選的機密，填入新的 `api-key`，檢查後套用，再按服務頁上方的**重新啟動**，讓副本讀到新值。

詳見 [OPENAI_API_KEY](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/model-provider.mdx)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 打開 `lobechat` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 時，用 `kubectl -n <namespace> logs deployment/lobechat` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### 對話失敗，出現模型服務商的錯誤

伺服器用你在安裝表單選的機密變數裡的 `api-key`，把對話送到 OpenAI。對話失敗時，先看 `lobechat` 服務的**日誌**分頁，再確認三件事。金鑰要有效。你選的模型要在這把金鑰的可用範圍內。你的帳號在 OpenAI 也不能超過頻率限制或額度。詳見 LobeChat 文件的 [OPENAI_API_KEY](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/model-provider.mdx)。
