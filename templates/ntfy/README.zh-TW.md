## 簡介

[ntfy](https://ntfy.sh/) 讓腳本和應用程式透過一般的 HTTP 請求，把推播通知送到你的手機或桌面。你把訊息發到某個主題，訂閱這個主題的所有用戶端都會收到。

這個範本在你的網域後面跑一個 ntfy 伺服器。每個主題都需要帳號，訊息和使用者存在 5 GiB 的磁碟區。每次啟動都會建好名為 `admin` 的使用者。

## 開始使用

1. 在 Zeabur 打開 `ntfy` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `ntfy.example.com`。
2. 以 `admin` 登入。密碼從 `ntfy` 服務取得：打開**環境變數**，在**來自設定檔和機密變數**點 `ntfy-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `ntfy-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret ntfy-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 所有主題預設都拒絕存取，所以要發送或訂閱的應用程式，必須用這個帳號或存取權杖登入。請見「設定」。

## 設定

### 更換 admin 密碼

ntfy 每次啟動都會套用這個機密變數。要換密碼，在 `ntfy` 服務的**環境變數**，於**來自設定檔和機密變數**點 `ntfy-secrets`，填入新的 `admin-password`，檢查後套用，再按服務頁上方的**重新啟動**。

### 新增使用者

按 `ntfy` 服務頁上方的**終端機**。執行 `ntfy user add`，再用 `ntfy access` 讓使用者能存取某個主題。`rw` 表示可以發送也可以訂閱：

```sh
ntfy user add <使用者名稱>
ntfy access <使用者名稱> <主題> rw
```

詳見 ntfy 文件的 [Access control](https://docs.ntfy.sh/config/)。

### 建立存取權杖

存取權杖讓應用程式不用你的密碼就能發送或訂閱。登入後在網頁應用程式的 Account 區域建立。也可以按 `ntfy` 服務頁上方的**終端機**，執行：

```sh
ntfy token add --label=<標籤> admin
```

把權杖當作 Bearer 權杖送出：

```sh
curl -H "Authorization: Bearer tk_..." -d "Hello" https://ntfy.example.com/mytopic
```

詳見 ntfy 文件的 [Publishing](https://docs.ntfy.sh/publish/)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 打開 `ntfy` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

詳見 Zeabur 文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### 用戶端無法發送或訂閱

範本預設拒絕存取每個主題。請讓用戶端用使用者名稱和密碼，或存取權杖登入。`admin` 以外的使用者，還要用 `ntfy access` 讓它能存取該主題。詳見 ntfy 文件的 [Access control](https://docs.ntfy.sh/config/)。
