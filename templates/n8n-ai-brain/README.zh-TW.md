## 簡介

[n8n](https://github.com/n8n-io/n8n) 是工作流程自動化工具。你用節點把應用程式和服務串起來，n8n 會依觸發條件或排程執行工作流程。

這個範本在你的網域後面跑一個 n8n，並附帶專屬的 PostgreSQL 資料庫。Code 節點在獨立的 `n8n-runners` Pod 裡執行，這個 Pod 也能執行 Python。擁有者帳號會在啟動時建好，所以不會出現設定畫面。

## 開始使用

1. 在 Zeabur 打開 `n8n` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `n8n.example.com`。
2. 用你在安裝表單填的擁有者 Email 登入。密碼從 `n8n` 服務取得：打開**環境變數**，在**來自設定檔和機密變數**點 `app-secrets`，再按 `owner-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `app-secrets` 的 `owner-password`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. 把加密金鑰另外備份到安全的地方。n8n 用這把金鑰加密它儲存的憑證。取得方式相同：在**環境變數**點 `app-secrets`，再按 `encryption-key` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `app-secrets` 的 `encryption-key`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. 機密 `app-secrets` 裡還有兩個日常使用不需要的欄位。`runner-token` 是 n8n 和程式碼執行器彼此信任用的共用機密，`jwt-key` 是使用者管理用的機密。取得方式同上。不用 Zeabur 時，用 kubectl：

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

## 疑難排解

### Code 節點不執行

Code 節點在 `n8n-runners` 服務裡執行，它用 `runner-token` 連到 n8n。在 Zeabur 打開 `n8n-runners` 服務，確認**概覽**顯示副本已就緒，再看**日誌**分頁。

詳見 n8n 文件的 [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners)。

### Code 節點裡的程式連不到網路

`n8n-runners` Pod 只連得到 DNS 和 n8n 的任務代理，因為範本用 NetworkPolicy 限制了它的網路。在 Code 節點裡呼叫外部位址的程式會失敗。請改用 HTTP Request 節點取得資料，再交給 Code 節點。
