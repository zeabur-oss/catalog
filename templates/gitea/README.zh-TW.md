## 簡介

[Gitea](https://about.gitea.com/) 是 Git 代管服務，提供儲存庫、議題和合併請求。這個範本在你的網域後面跑 Gitea，儲存庫放在磁碟區，搭配 PostgreSQL 資料庫。

公開註冊已關閉，SSH 也關閉了。大家用 HTTPS 使用 Gitea。

## 開始使用

1. 在 Zeabur 打開 `gitea` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `git.example.com`。不會出現安裝精靈。
2. 第一次啟動時會建立管理員 `gitea_admin`。取得它的密碼。打開**環境變數**，在**來自設定檔和機密變數**點 `gitea-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `gitea-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret gitea-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 用 `gitea_admin` 和這組密碼登入，並在 Gitea 裡更改密碼。Gitea 只在第一次啟動時讀取這個機密，之後再改機密不會生效。

`gitea-secrets` 裡其他的欄位（`secret-key`、`internal-token`、`jwt-secret` 和 `lfs-jwt-secret`）是 Gitea 自己使用。`secret-key` 要在整個資料庫的使用期間保留，因為 Gitea 用它加密雙因素驗證密鑰等已儲存的認證資訊。

## 設定

### 建立使用者

註冊已關閉，其他使用者由管理員建立。開啟 Site Administration，再開啟 User Accounts。詳見 Gitea 的 [FAQ](https://docs.gitea.com/help/faq)。

### 用存取權杖透過 HTTPS 推送

在 Settings > Applications 建立存取權杖，也可以直接開啟 `https://git.example.com/user/settings/applications`。給它需要的權限，並在 Gitea 顯示時複製下來，因為 Gitea 只顯示一次。Git 要求密碼時，把權杖當作密碼輸入：

```sh
git clone https://git.example.com/<使用者>/<儲存庫>.git
```

詳見 [API Usage](https://docs.gitea.com/development/api-usage)。

### 備份資料

Gitea 把帳號、議題和設定存在 `gitea-db` 資料庫。在專案打開 `gitea-db`，到**備份**設定 S3 備份。想直接拿到檔案，按**更多** → **匯出**。儲存庫和上傳的檔案放在磁碟區 `gitea-data`，掛載在 `/var/lib/gitea`。用 `gitea` 服務的**檔案**分頁複製。資料夾很大時，改用 kubectl。`kubectl -n <namespace> get pods` 會列出副本名稱：

```sh
kubectl -n <namespace> cp <replica>:/var/lib/gitea ./gitea-data
```

Gitea 也有 `gitea dump` 指令，把儲存庫、資料庫和資料打包成一個 ZIP 檔。Gitea 建議備份期間先停止執行個體，讓檔案和資料庫保持一致。詳見 Zeabur 文件的[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)、[下載資料庫傾印檔](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#下載資料庫傾印檔)，以及 Gitea 文件的 [Backup and Restore](https://docs.gitea.com/administration/backup-and-restore)。

## 疑難排解

### 用 SSH 複製儲存庫失敗

這個範本關閉了 SSH。請使用儲存庫的 HTTPS 複製網址。詳見 [Config Cheat Sheet](https://docs.gitea.com/administration/config-cheat-sheet)。

### 頁面顯示 502 或打不開

在 Zeabur 打開 `gitea` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 時，用 `kubectl -n <namespace> logs deployment/gitea` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。
