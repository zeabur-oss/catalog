## 簡介

[WordPress](https://wordpress.org/) 是網站和部落格引擎。這個範本在你的網域後面跑 WordPress，並附帶它自己的 MariaDB 資料庫。網站檔案和上傳的檔案存在磁碟區裡，資料庫則有自己的 10Gi 磁碟區。

網站已經安裝好。擁有者帳號 `admin` 已建好，管理員 Email 就是你在安裝表單填的那一個。允許上傳最大 256 MB 的檔案。

## 開始使用

1. 在 Zeabur 打開 `wordpress` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，也就是你在安裝表單填的網域。
2. 在 `https://<你的網域>/wp-admin` 用 `admin` 登入。打開**環境變數**，在**來自設定檔和機密變數**點 `wordpress-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `wordpress-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 資料庫是 MariaDB，在專案裡是 `mariadb` 資料庫。WordPress 連到 Service `mariadb` 的 `3306` 連接埠，資料庫是 `zeabur`，使用者是 `wordpress`，連線使用 TLS。密碼在同一個機密變數的 `database-password` 和 `database-root-password`。用同樣的方式打開，或用 kubectl 讀取：

   ```sh
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.database-password}' | base64 -d
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.database-root-password}' | base64 -d
   ```

4. 同一個機密變數還有 WordPress 用來簽署登入 cookie 的金鑰和 salt：`auth-key`、`secure-auth-key`、`logged-in-key`、`nonce-key`、`auth-salt`、`secure-auth-salt`、`logged-in-salt` 和 `nonce-salt`。使用網站時用不到。要讀取時用同樣的指令，例如 `{.data.auth-key}`。

## 設定

### 更改管理員密碼

登入後，進入 Users、All Users，點你的使用者名稱。在 New Password 底下點 Generate Password，或自己輸入新密碼，再點 Update User。新密碼立刻生效。`wordpress-secrets` 裡的值只在第一次安裝時使用，所以不會跟著改變。詳見 [Reset your password](https://wordpress.org/documentation/article/reset-your-password/)。

### 選擇佈景主題和新增外掛

進入 Appearance、Themes 更換網站外觀，進入 Plugins 新增功能。詳見 [First steps with WordPress](https://wordpress.org/documentation/article/first-steps-with-wordpress/)。

### 設定頁面的網址格式

進入 Settings、Permalinks，選一種結構，再點 Save Changes。詳見 [Settings Permalinks screen](https://wordpress.org/documentation/article/settings-permalinks-screen/)。

### 備份資料

在 Zeabur 打開 `mariadb` 資料庫，到**備份** → **設定備份**，就能依排程備份到 S3 儲存貯體。要現在就拿一份資料，在資料庫頁面右上角按**更多** → **匯出**，瀏覽器會下載 `mariadb-dump --single-transaction` 產生、gzip 壓縮的 SQL。詳見[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)和[下載資料庫傾印檔](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#下載資料庫傾印檔)。

上傳的檔案、佈景主題和外掛不在資料庫裡，而是在 `wordpress-data` 磁碟區，掛載在 `wordpress` 服務的 `/var/www/html`。請把 `wp-content` 和資料庫傾印檔一起備份。在 Zeabur 可以用**檔案**分頁下載資料夾，但資料夾很大時可能逾時。`wp-content` 很大時，改用 kubectl 串流輸出：

```sh
kubectl -n <namespace> exec deploy/wordpress -c wordpress -- tar czf - -C /var/www/html wp-content > wp-content.tar.gz
```

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 打開 `wordpress` 服務，確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本一直重新啟動時，打開**部署紀錄**，切到**事件**。如果事件寫著 `OOMKilled`，到**設定** → **運算資源**調高**記憶體上限**。詳見[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

網域或憑證一直停在等待中時，看[加上公開網域與 HTTPS](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)的「遇到問題」。
