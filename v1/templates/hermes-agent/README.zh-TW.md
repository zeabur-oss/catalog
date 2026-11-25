## 簡介

[Hermes Agent](https://github.com/NousResearch/hermes-agent) 是透過 API 和網頁管理面板運作的 AI 代理。API 採用 OpenAI 格式，所以相容 OpenAI 的用戶端都能把這個代理當後端。

這個範本用兩個網域執行 Hermes Agent，一個給 API，一個給管理面板。代理透過 OpenRouter，或你在安裝表單填的 OpenAI 相容端點呼叫模型。

## 開始使用

1. 在 Zeabur 開源版打開 `hermes-agent` 服務。**概覽**顯示副本已就緒，服務就啟動好了。兩個網域都在**網路**分頁的**公開網域**，例如 API 用 `hermes.example.com`，管理面板用 `hermes-admin.example.com`。
2. 取得管理面板密碼。打開**環境變數**，在**來自設定檔和機密變數**點 `hermes-secrets`，再按 `dashboard-password` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `hermes-secrets` 的 `dashboard-password`：

   ```sh
   kubectl -n <namespace> get secret hermes-secrets -o jsonpath='{.data.dashboard-password}' | base64 -d
   ```

3. 開啟管理面板的網域，用 `admin` 和這組密碼登入。
4. 取得 API 金鑰。在同一個 `hermes-secrets` 列表按 `api-key` 旁的眼睛圖示。用 kubectl 的做法：

   ```sh
   kubectl -n <namespace> get secret hermes-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

5. 在 `https://hermes.example.com/v1` 呼叫 API，並把金鑰當作 Bearer 權杖送出：

   ```sh
   curl https://hermes.example.com/v1/chat/completions \
     -H "Authorization: Bearer <API 金鑰>" \
     -H "Content-Type: application/json" \
     -d '{"model": "hermes-agent", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

模型服務的 API 金鑰不在 `hermes-secrets`，而是在你於安裝表單選的機密的 `api-key` 欄位。`hermes-secrets` 的 `dashboard-session-key` 用來簽署管理面板的工作階段。

## 設定

### 連接用戶端

Open WebUI、LobeChat、LibreChat 等相容 OpenAI 的用戶端都能使用這個代理。把 API 基礎網址設為 `https://hermes.example.com/v1`，API 金鑰填 `hermes-secrets` 裡 `api-key` 的值。詳見 [API Server](https://hermes-agent.nousresearch.com/docs/user-guide/features/api-server)。

### 更換模型服務的 API 金鑰

在服務的**環境變數**分頁，於**來自設定檔和機密變數**點你在安裝表單選的機密，填入新的 `api-key`，檢查後套用。執行中的副本會繼續用舊金鑰，直到你按服務頁上方的**重新啟動**。

範本每次啟動都會把安裝表單填的模型、端點和金鑰寫進 Hermes 的設定。在管理面板更改模型，只會維持到下次重新啟動。

管理面板能管理哪些項目，詳見 [Web Dashboard](https://hermes-agent.nousresearch.com/docs/user-guide/features/web-dashboard)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `hermes-agent` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 開源版時，用 `kubectl -n <namespace> logs deployment/hermes-agent` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 開源版文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### 請求失敗，出現模型服務商的錯誤

Hermes 用安裝表單填的模型名稱、端點和金鑰呼叫模型。呼叫失敗時，先看 `hermes-agent` 服務的**日誌**分頁，再確認三件事。模型名稱要存在於你的服務商。你在安裝表單選的機密變數裡，`api-key` 要有效。你的帳號在服務商那裡也不能超過頻率限制或額度。詳見 Hermes 文件的 [AI Providers](https://hermes-agent.nousresearch.com/docs/integrations/providers)。
