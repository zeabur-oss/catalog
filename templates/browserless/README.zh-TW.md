## 簡介

[Browserless](https://github.com/browserless/browserless) 把無頭 Chrome 變成一個服務。Puppeteer、Playwright 等自動化工具透過 WebSocket 或 HTTP 連上它，自己的機器上就不用裝瀏覽器。

這個範本在你的網域後面跑一個 Browserless。每次呼叫都要帶權杖，權杖在部署時自動產生。

## 開始使用

1. 在 Zeabur 打開 `browserless` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `browserless.example.com`。
2. 取得權杖。打開**環境變數**，在**來自設定檔和機密變數**點 `browserless-secrets`，再按 `token` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `browserless-secrets` 的 `token`：

   ```sh
   kubectl -n <namespace> get secret browserless-secrets -o jsonpath='{.data.token}' | base64 -d
   ```

3. 把用戶端指向這個網域，並在 `Authorization: Bearer <token>` 標頭帶上權杖。用戶端不能設標頭時，在網址加上 `?token=<token>`。以 Puppeteer 為例：

   ```js
   const browser = await puppeteer.connect({
     browserWSEndpoint: "wss://browserless.example.com?token=<token>",
   });
   ```

   這個服務沒有網頁，也不用登入。沒帶權杖的呼叫會得到 `401`。

## 設定

### 從 Playwright 連線

使用 `chromium.connectOverCDP("wss://<你的網域>?token=<token>")`。詳見 Browserless 文件的 [Playwright connection](https://docs.browserless.io/examples/playwright-connection)。

### 更換權杖

在服務的**環境變數**分頁點 `browserless-secrets`，填入新的 `token`，檢查後套用。執行中的副本會繼續用舊權杖，直到你按服務頁上方的**重新啟動**。所有用到舊權杖的用戶端都要跟著更新。

## 疑難排解

### 工作階段在排隊或逾時

範本限制同時開啟的瀏覽器數量（`CONCURRENT`），其餘的會排隊（`QUEUED`）。先關掉用戶端不再需要的瀏覽器；要調高這兩個值之前，先調高服務的記憶體上限。詳見 [Docker configuration](https://docs.browserless.io/baas/docker/config)。
