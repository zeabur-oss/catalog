## 簡介

[New API](https://github.com/QuantumNous/new-api) 是 AI 模型的閘道。它給你的應用程式一組相容 OpenAI 的 API，並把每個請求送到你設定好的模型服務商。

這個範本在你的網域後面跑一個 New API，並附帶專屬的 PostgreSQL 資料庫。初始設定已經替你完成，所以不會出現設定頁面，名為 `admin` 的管理員帳號也已建好。

## 開始使用

1. 在 Zeabur 開源版打開 `new-api` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `newapi.example.com`。
2. 以 `admin` 登入。密碼是 `Aa1-` 加上 `admin-password` 的值。值從 `new-api` 服務取得：打開**環境變數**，在**來自設定檔和機密變數**點 `newapi-secrets`，再按 `admin-password` 旁的眼睛圖示，並在前面加上 `Aa1-`。不用 Zeabur 開源版時，下面的 kubectl 指令會印出完整的密碼：

   ```sh
   echo "Aa1-$(kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.admin-password}' | base64 -d)"
   ```

3. 把你的模型服務商加成管道（channel）。詳見「設定」。
4. 機密 `newapi-secrets` 裡還有兩個日常使用不需要的欄位。`session-secret` 是工作階段機密，`crypto-secret` 是快取鍵用的 HMAC 機密。取得方式同上。不用 Zeabur 開源版時，用 kubectl：

   ```sh
   kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.session-secret}' | base64 -d
   kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.crypto-secret}' | base64 -d
   ```

## 設定

### 新增模型服務商

管道（channel）是對一個模型服務商的一條連線。以 `admin` 登入後，開啟左側選單的 Channels，選 Add Channel。選擇服務商類型，填入名稱和服務商的 API 金鑰，並勾選這個管道支援的模型。用管道上的 Test 確認能正常運作。詳見 New API 文件的 [Channel Management](https://docs.newapi.ai/en/docs/guide/feature-guide/admin/channel)。

### 從應用程式呼叫閘道

在主控台建立 API 權杖，詳見 [API Tokens](https://docs.newapi.ai/en/docs/guide/console/api-token)。接著把應用程式指向 `https://<你的網域>/v1`，並把權杖當作 Bearer 權杖送出：

```sh
curl https://newapi.example.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-<token>" \
  -d '{"model":"<model>","messages":[{"role":"user","content":"Hello"}]}'
```

詳見 New API 文件的 [Using API and Playground](https://docs.newapi.ai/en/docs/guide/feature-guide/user/api)。

### 更改管理員密碼

New API 只在第一次啟動、建立管理員時讀取 `admin-password`。之後改機密變數不會改變密碼。請登入 New API 後在裡面更改。

### 備份資料

New API 把存下來的東西都放在 `newapi-db` 資料庫。範本只把日誌檔寫到 `/data`，那是暫存空間，替換副本時會被刪除，所以沒有磁碟區要備份。在專案打開 `newapi-db` 資料庫。到**備份**設定 S3 備份。想直接拿到檔案，按**更多** → **匯出**。詳見 Zeabur 開源版文件的[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)和[下載資料庫傾印檔](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#下載資料庫傾印檔)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `new-api` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

詳見 Zeabur 開源版文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### 用自動產生的密碼登入失敗

密碼不只是機密的值，它是以 `Aa1-` 開頭的。請用「開始使用」裡的指令印出完整的密碼。

### 請求出現服務商的錯誤

New API 會把服務商的錯誤傳回來。以 `admin` 開啟 Channels，檢查三件事。請求裡的模型名稱必須是管道列出的模型，也必須是服務商有的模型。管道的 API 金鑰必須有效，可用管道上的 Test 確認。服務商的速率限制或額度不能已經用完。接著在 Zeabur 開源版打開 `new-api` 服務的**日誌**分頁，讀錯誤訊息。詳見 New API 文件的 [Channel Management](https://docs.newapi.ai/en/docs/guide/feature-guide/admin/channel)。
