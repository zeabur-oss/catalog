## 簡介

[n8n](https://github.com/n8n-io/n8n) 是工作流程自動化工具。你用節點把應用程式和服務串起來，n8n 會依觸發條件或排程執行工作流程。

這個範本在你的網域後面跑一個 n8n，資料用 SQLite 存在磁碟區。Code 節點在獨立的 `task-runner` Pod 裡執行。擁有者帳號會在啟動時建好，所以不會出現設定畫面。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://n8n.example.com`。
2. 用你在安裝表單填的擁有者 Email 登入。密碼在機密 `n8n-secrets` 的 `owner-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. 把加密金鑰另外備份到安全的地方。n8n 用這把金鑰加密它儲存的憑證。金鑰在機密 `n8n-secrets` 的 `encryption-key` 欄位：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. 機密 `n8n-secrets` 裡還有 `runner-token`，這是 n8n 和 task runner 彼此信任用的共用機密。日常使用不需要它：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.runner-token}' | base64 -d
   ```

## 設定

### 更換擁有者密碼

n8n 每次啟動都會用安裝表單和機密設定擁有者帳號，並在介面裡把擁有者資料顯示為唯讀。要換密碼，把新的值寫進 `n8n-secrets` 的 `owner-password` 欄位，再重新啟動 Deployment：

```sh
kubectl -n <namespace> patch secret n8n-secrets --type merge -p '{"stringData":{"owner-password":"<新密碼>"}}'
kubectl -n <namespace> rollout restart deployment/n8n
```

詳見 n8n 文件的 [Change the instance owner email](https://docs.n8n.io/deploy/host-n8n/configure-n8n/change-instance-owner-email)。

### 用 Webhook 呼叫工作流程

Webhook 節點有兩個網址。測試網址在你選 Listen for Test Event 時有效，並會在編輯器顯示收到的資料。正式網址在你發布工作流程後有效。兩個網址都以你的網域開頭。詳見 n8n 文件的 [Webhook node](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook/)。

## 疑難排解

### Code 節點不執行

Code 節點在 `task-runner` Pod 裡執行，這個 Pod 用 `runner-token` 連回 n8n。請確認 Pod 在執行中，並查看它的記錄：

```sh
kubectl -n <namespace> get pods
kubectl -n <namespace> logs deployment/task-runner
```

詳見 n8n 文件的 [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners)。
