## 簡介

[OpenClaw](https://github.com/openclaw/openclaw) 是個人 AI 助理。Gateway（閘道）負責執行助理，網頁控制介面（Control UI）讓你和它對話並管理它。它也能串接 WhatsApp、Telegram、Slack、Discord 等通訊軟體。

這個範本在你的網域後面跑一個 Gateway，使用你在安裝表單填的、相容 OpenAI 的模型服務商。

## 開始使用

1. 在 Zeabur 開源版打開 `openclaw` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `openclaw.example.com`。
2. 登入畫面有一個 Gateway secret 欄位，把閘道權杖貼進去。權杖從 `openclaw` 服務取得：打開**環境變數**，在**來自設定檔和機密變數**點 `openclaw-secrets`，再按 `gateway-token` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `openclaw-secrets` 的 `gateway-token`：

   ```sh
   kubectl -n <namespace> get secret openclaw-secrets -o jsonpath='{.data.gateway-token}' | base64 -d
   ```

3. 新的瀏覽器在你放行一次之前，會顯示 `disconnected (1008): pairing required`。請保持頁面開啟。按 `openclaw` 服務頁上方的**終端機**，列出請求，再放行你的那一筆。放行後頁面會自己連線：

   ```sh
   node /app/openclaw.mjs devices list
   node /app/openclaw.mjs devices approve <requestId>
   ```

4. 模型服務商就是你在安裝表單填的 base URL、模型和 API 金鑰。API 金鑰在你於表單裡指定的機密變數，欄位是 `api-key`，不在 `openclaw-secrets` 裡。要查看，打開 `openclaw` 服務的**環境變數**，點那個機密變數。

## 設定

### 更換服務商的 API 金鑰

在 `openclaw` 服務打開**環境變數**，點你在安裝表單指定的機密變數。填入新的 `api-key`，檢查後套用，再按服務頁上方的**重新啟動**，讓 Gateway 讀到新值。

base URL 和模型只在第一次啟動時寫進 Gateway 的設定檔。之後再改安裝表單，不會改變它們。

### 移除已配對的瀏覽器

按 `openclaw` 服務頁上方的**終端機**。列出已配對的裝置，再移除不用的那一個：

```sh
node /app/openclaw.mjs devices list
node /app/openclaw.mjs devices remove <deviceId>
```

詳見 OpenClaw 文件的 [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `openclaw` 服務，確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本一直重新啟動時，打開**部署紀錄**，切到**事件**。如果事件寫著 `OOMKilled`，到**設定** → **運算資源**調高**記憶體上限**。詳見[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

服務變慢或一直重新連線時，也照這些步驟檢查。

網域或憑證一直停在等待中時，看[加上公開網域與 HTTPS](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)的「遇到問題」。

### 另一個瀏覽器要求配對

每個瀏覽器設定檔各自配對，一個設定檔不能沿用另一個的放行。請在**終端機**對每個設定檔用 `devices approve` 放行一次，並用 `devices remove` 移除舊的項目。詳見 OpenClaw 文件的 [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair)。

### 顯示 All models failed 或模型服務回傳錯誤

OpenClaw 用安裝表單填的 API 網址和模型送出模型請求。依序檢查：

- 模型名稱在你的模型服務上確實存在。名稱填錯很常見。
- 你在安裝表單指定的機密變數裡，鍵 `api-key` 的 API 金鑰有效，也沒有超過模型服務的速率限制或額度。
- API 網址是 OpenAI 相容 API 的 HTTPS 位址。

打開 `openclaw` 服務的**日誌**分頁，看模型服務回傳的錯誤。API 網址和模型只在第一次啟動時從安裝表單讀取。之後要改，用控制介面的 **Config** 分頁。API 網址是 `models.providers.custom.baseUrl`，模型名稱要在 `models.providers.custom.models` 和 `agents.defaults.model.primary` 兩處都改。詳見 OpenClaw 文件的 [Model providers](https://docs.openclaw.ai/concepts/model-providers) 和 [Configuration](https://docs.openclaw.ai/gateway/configuration)。
