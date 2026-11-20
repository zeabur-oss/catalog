## 簡介

[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 是 API 代理。用戶端把 OpenAI 或 Gemini 格式的請求送給它，它再轉給你設定好的模型服務。

這個範本在你的網域後面跑一個 CLIProxyAPI。API 由金鑰保護，金鑰在部署時自動產生。它沒有網頁，也不用登入。管理 API 不在網域上。

## 開始使用

1. 在 Zeabur 打開 `cliproxyapi` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `cliproxyapi.example.com`。
2. 取得 API 金鑰。打開**環境變數**，在**來自設定檔和機密變數**點 `app-secrets`，再按 `api-key` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `app-secrets` 的 `api-key`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.api-key}' | base64 -d
   ```

3. 把用戶端指向 `https://cliproxyapi.example.com/v1`（Gemini 格式的用戶端用 `/v1beta`），並把金鑰當作 Bearer 權杖送出。沒帶金鑰的呼叫會得到 `401`。
4. 需要管理金鑰時再取得。它是同一個 `app-secrets` 裡的 `management-key`，也可以用 kubectl 讀取：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.management-key}' | base64 -d
   ```

API 金鑰只管用戶端進來。模型服務的認證資訊要另外設定，見「設定」。

## 設定

### 呼叫管理 API

管理 API 只接受來自 Pod 內部的呼叫。Zeabur 沒有對應的畫面，請先用 `kubectl` 開一條通道，再把管理金鑰當作 Bearer 權杖送出：

```sh
kubectl -n <namespace> port-forward deployment/cliproxyapi 8317:8317 --address 127.0.0.1
curl -H 'Authorization: Bearer <管理金鑰>' http://localhost:8317/v8/management/config
```

詳見 [Management API v8](https://help.router-for.me/management/apiv8.html)。

### 加入模型服務的認證檔

上傳 CLIProxyAPI 登入指令產生的認證檔。先開好上一節的通道：

```sh
curl -X POST -H 'Authorization: Bearer <管理金鑰>' \
  -F file=@<認證檔>.json \
  http://localhost:8317/v8/management/credentials
```

檔案存在磁碟區上，重新啟動後仍在。詳見 [Management API v8](https://help.router-for.me/management/apiv8.html) 和 [Authentication Directory](https://help.router-for.me/configuration/auth-dir.html)。

透過 `/config` 寫入的模型服務金鑰只會保留到 Pod 重新啟動，因為範本每次啟動都會重新產生設定檔。

## 疑難排解

### 管理 API 回傳 401 或不再回應

同一個位址連續送錯五次金鑰，會被封鎖約 30 分鐘。請確認用的是 `management-key` 的值，不是 `api-key`，並等封鎖結束。詳見 [Management API v8](https://help.router-for.me/management/apiv8.html)。

### 頁面顯示 502 或打不開

在 Zeabur 打開 `cliproxyapi` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 時，用 `kubectl -n <namespace> logs deployment/cliproxyapi` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### 請求失敗，出現模型服務商的錯誤

`api-key` 只負責讓你的用戶端進來，模型服務商要另外有自己的認證檔，見「加入模型服務的認證檔」。請求失敗時，先看 `cliproxyapi` 服務的**日誌**分頁，再確認三件事。請求裡的模型名稱要是你加進去的某個認證檔能用的模型。那個認證檔要還有效。它的帳號在服務商那裡也不能超過頻率限制或額度。詳見 CLIProxyAPI 文件的 [Authentication Directory](https://help.router-for.me/configuration/auth-dir.html)。
