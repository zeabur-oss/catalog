## 簡介

[n8n](https://github.com/n8n-io/n8n) 是工作流程自動化工具。你用節點把應用程式和服務串起來，n8n 會依觸發條件或排程執行工作流程。

這個範本在你的網域後面跑一個 n8n，資料用 SQLite 存在磁碟區。Code 節點在獨立的 `task-runner` Pod 裡執行。擁有者帳號會在啟動時建好，所以不會出現設定畫面。

## 開始使用

1. 在 Zeabur 打開 `n8n` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `n8n.example.com`。
2. 用你在安裝表單填的擁有者 Email 登入。密碼從 `n8n` 服務取得：打開**環境變數**，在**來自設定檔和機密變數**點 `n8n-secrets`，再按 `owner-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `n8n-secrets` 的 `owner-password`：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. 把加密金鑰另外備份到安全的地方。n8n 用這把金鑰加密它儲存的憑證。取得方式相同：在**環境變數**點 `n8n-secrets`，再按 `encryption-key` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `n8n-secrets` 的 `encryption-key`：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. 機密 `n8n-secrets` 裡還有 `runner-token`，這是 n8n 和 task runner 彼此信任用的共用機密。日常使用不需要它。取得方式同上。不用 Zeabur 時，用 kubectl：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.runner-token}' | base64 -d
   ```

## 設定

### 更換擁有者密碼

n8n 每次啟動都會用安裝表單和機密變數設定擁有者帳號，並在介面裡把擁有者資料顯示為唯讀。要換密碼，在 `n8n` 服務的**環境變數**，於**來自設定檔和機密變數**點 `n8n-secrets`，填入新的 `owner-password`，檢查後套用，再按服務頁上方的**重新啟動**。

詳見 n8n 文件的 [Change the instance owner email](https://docs.n8n.io/deploy/host-n8n/configure-n8n/change-instance-owner-email)。

### 用 Webhook 呼叫工作流程

Webhook 節點有兩個網址。測試網址在你選 Listen for Test Event 時有效，並會在編輯器顯示收到的資料。正式網址在你發布工作流程後有效。兩個網址都以你的網域開頭。詳見 n8n 文件的 [Webhook node](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook/)。

### 備份或搬移資料

n8n 把 SQLite 資料庫和檔案放在 `n8n-data` 磁碟區，掛載在 `/home/node/.n8n`。n8n 建議先停掉它再複製 SQLite 檔，所以請改用 n8n 的命令列匯出工作流程和憑證。

1. 留好「開始使用」裡的加密金鑰。沒有它，n8n 讀不了已儲存的憑證。
2. 匯出工作流程和憑證。按 `n8n` 服務頁上方的**終端機**，執行：

   ```sh
   n8n export:workflow --backup --output=/home/node/.n8n/backups/workflows/
   n8n export:credentials --backup --output=/home/node/.n8n/backups/credentials/
   ```

   接著到 `n8n` 服務的**檔案**分頁，下載這兩個資料夾。不要下載整個磁碟區，資料夾太大會逾時。

3. 複製工作流程存下的檔案。這個模板把它們放在磁碟區的 `/home/node/.n8n/binaryData`，資料庫和匯出都不包含它們。這個資料夾可能很大，所以用 kubectl 直接傳到你的電腦，不要用**檔案**分頁：

   ```sh
   kubectl -n <namespace> exec deploy/n8n -c n8n -- busybox tar czf - -C /home/node/.n8n binaryData > n8n-binary-data.tgz
   ```

   要放回新的 `n8n`，執行 `kubectl -n <namespace> exec -i deploy/n8n -c n8n -- busybox tar xzf - -C /home/node/.n8n < n8n-binary-data.tgz`。詳見 n8n 文件的 [Binary data environment variables](https://docs.n8n.io/hosting/configuration/environment-variables/binary-data/)。

要搬到另一個 n8n，先在新 `n8n` 服務的**檔案**分頁上傳這兩個資料夾，再用**終端機**匯入：

```sh
n8n import:workflow --separate --input=<workflows folder>
n8n import:credentials --separate --input=<credentials folder>
```

新的 n8n 必須用同一把加密金鑰，否則讀不了憑證。做不到時，在 `n8n export:credentials` 加上 `--decrypted`。這個檔案裡的憑證是明文，匯入後要刪掉。匯出的內容不含使用者、執行紀錄、變數和實例設定。

詳見 n8n 文件的 [Back up and restore](https://docs.n8n.io/deploy/host-n8n/keep-n8n-running/backup-and-restore) 和 [Use the command line](https://docs.n8n.io/deploy/host-n8n/configure-n8n/use-the-command-line)。

## 疑難排解

### 頁面顯示 502、打不開或一直重新連線

在 Zeabur 打開 `n8n` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。服務很慢或一直重新連線，也常是記憶體不夠，同樣檢查**事件**和**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

詳見 Zeabur 文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### Code 節點不執行

Code 節點在 `task-runner` 服務裡執行，它用 `runner-token` 連到 n8n。在 Zeabur 打開 `task-runner` 服務，確認**概覽**顯示副本已就緒，再看**日誌**分頁。

詳見 n8n 文件的 [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners)。
