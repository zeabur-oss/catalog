## 簡介

[n8n](https://github.com/n8n-io/n8n) 是工作流程自動化工具。你用節點把應用程式和服務串起來，n8n 會依觸發條件或排程執行工作流程。

這個範本在你的網域後面跑一個 n8n，並附帶專屬的 PostgreSQL 資料庫。Code 節點在獨立的 `n8n-runners` Pod 裡執行，這個 Pod 也能執行 Python。擁有者帳號會在啟動時建好，所以不會出現設定畫面。

## 開始使用

1. 在 Zeabur 開源版打開 `n8n` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `n8n.example.com`。
2. 用你在安裝表單填的擁有者 Email 登入。密碼從 `n8n` 服務取得：打開**環境變數**，在**來自設定檔和機密變數**點 `app-secrets`，再按 `owner-password` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `app-secrets` 的 `owner-password`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. 把加密金鑰另外備份到安全的地方。n8n 用這把金鑰加密它儲存的憑證。取得方式相同：在**環境變數**點 `app-secrets`，再按 `encryption-key` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `app-secrets` 的 `encryption-key`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. 機密 `app-secrets` 裡還有兩個日常使用不需要的欄位。`runner-token` 是 n8n 和程式碼執行器彼此信任用的共用機密，`jwt-key` 是使用者管理用的機密。取得方式同上。不用 Zeabur 開源版時，用 kubectl：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.runner-token}' | base64 -d
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.jwt-key}' | base64 -d
   ```

## 設定

### 更換擁有者密碼

n8n 每次啟動都會用安裝表單和機密變數設定擁有者帳號，並在介面裡把擁有者資料顯示為唯讀。要換密碼，在 `n8n` 服務的**環境變數**，於**來自設定檔和機密變數**點 `app-secrets`，填入新的 `owner-password`，檢查後套用，再按服務頁上方的**重新啟動**。

詳見 n8n 文件的 [Change the instance owner email](https://docs.n8n.io/deploy/host-n8n/configure-n8n/change-instance-owner-email)。

### 用 Webhook 呼叫工作流程

Webhook 節點有兩個網址。測試網址在你選 Listen for Test Event 時有效，並會在編輯器顯示收到的資料。正式網址在你發布工作流程後有效。兩個網址都以你的網域開頭。詳見 n8n 文件的 [Webhook node](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook/)。

### 備份或搬移資料

n8n 把工作流程、憑證和執行紀錄存在 `n8n-db` 資料庫。`n8n-data` 磁碟區掛載在 `/home/node/.n8n`，放二進位資料之類的檔案。備份要有資料庫和加密金鑰，要搬到另一個 n8n 時，還要匯出工作流程和憑證。

1. 備份資料庫。在專案打開 `n8n-db` 資料庫。到**備份**設定 S3 備份。想直接拿到檔案，按**更多** → **匯出**。詳見 Zeabur 開源版文件的[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)和[下載資料庫傾印檔](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#下載資料庫傾印檔)。
2. 留好「開始使用」裡的加密金鑰。沒有它，n8n 讀不了已儲存的憑證。
3. 匯出工作流程和憑證。按 `n8n` 服務頁上方的**終端機**，執行：

   ```sh
   n8n export:workflow --backup --output=/home/node/.n8n/backups/workflows/
   n8n export:credentials --backup --output=/home/node/.n8n/backups/credentials/
   ```

   接著到 `n8n` 服務的**檔案**分頁，下載這兩個資料夾。不要下載整個磁碟區，資料夾太大會逾時。

要搬到另一個 n8n，先在新 `n8n` 服務的**檔案**分頁上傳這兩個資料夾，再用**終端機**匯入：

```sh
n8n import:workflow --separate --input=<workflows folder>
n8n import:credentials --separate --input=<credentials folder>
```

新的 n8n 必須用同一把加密金鑰，否則讀不了憑證。做不到時，在 `n8n export:credentials` 加上 `--decrypted`。這個檔案裡的憑證是明文，匯入後要刪掉。匯出的內容不含使用者、執行紀錄、變數和實例設定。

詳見 n8n 文件的 [Back up and restore](https://docs.n8n.io/deploy/host-n8n/keep-n8n-running/backup-and-restore) 和 [Use the command line](https://docs.n8n.io/deploy/host-n8n/configure-n8n/use-the-command-line)。

## 疑難排解

### 頁面顯示 502、打不開或一直重新連線

在 Zeabur 開源版打開 `n8n` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。服務很慢或一直重新連線，也常是記憶體不夠，同樣檢查**事件**和**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

詳見 Zeabur 開源版文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### Code 節點不執行

Code 節點在 `n8n-runners` 服務裡執行，它用 `runner-token` 連到 n8n。在 Zeabur 開源版打開 `n8n-runners` 服務，確認**概覽**顯示副本已就緒，再看**日誌**分頁。

詳見 n8n 文件的 [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners)。

### Code 節點裡的程式連不到網路

`n8n-runners` Pod 只連得到 DNS 和 n8n 的任務代理，因為範本用 NetworkPolicy 限制了它的網路。在 Code 節點裡呼叫外部位址的程式會失敗。請改用 HTTP Request 節點取得資料，再交給 Code 節點。
