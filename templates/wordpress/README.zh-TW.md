## 簡介

[WordPress](https://wordpress.org/) 是網站和部落格引擎。這個範本在你的網域後面跑 WordPress，並附帶它自己的 MariaDB 資料庫。網站檔案和上傳的檔案存在磁碟區裡，資料庫則有自己的 10Gi 磁碟區。

網站已經安裝好。擁有者帳號 `admin` 已建好，管理員 Email 就是你在安裝表單填的那一個。允許上傳最大 256 MB 的檔案。

## 開始使用

1. 等服務顯示為執行中，開啟 `https://<你的網域>/wp-admin`。
2. 用 `admin` 登入。密碼在機密 `wordpress-secrets` 的 `admin-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 資料庫是 MariaDB。WordPress 連到 Service `mariadb` 的 `3306` 連接埠，資料庫是 `zeabur`，使用者是 `wordpress`，連線使用 TLS。密碼在同一個機密的 `database-password` 和 `database-root-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.database-password}' | base64 -d
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.database-root-password}' | base64 -d
   ```

4. 同一個機密還有 WordPress 用來簽署登入 cookie 的金鑰和 salt：`auth-key`、`secure-auth-key`、`logged-in-key`、`nonce-key`、`auth-salt`、`secure-auth-salt`、`logged-in-salt` 和 `nonce-salt`。使用網站時用不到。要讀取時用同樣的指令，例如 `{.data.auth-key}`。

## 設定

### 更改管理員密碼

登入後，進入 Users、All Users，點你的使用者名稱。在 New Password 底下點 Generate Password，或自己輸入新密碼，再點 Update User。新密碼立刻生效。`wordpress-secrets` 裡的值只在第一次安裝時使用，所以不會跟著改變。詳見 [Reset your password](https://wordpress.org/documentation/article/reset-your-password/)。

### 選擇佈景主題和新增外掛

進入 Appearance、Themes 更換網站外觀，進入 Plugins 新增功能。詳見 [First steps with WordPress](https://wordpress.org/documentation/article/first-steps-with-wordpress/)。

### 設定頁面的網址格式

進入 Settings、Permalinks，選一種結構，再點 Save Changes。詳見 [Settings Permalinks screen](https://wordpress.org/documentation/article/settings-permalinks-screen/)。
