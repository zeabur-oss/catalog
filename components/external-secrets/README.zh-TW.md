## 簡介

External Secrets Operator 會把外部機密庫（例如 OpenBao、Vault、1Password 或 AWS Secrets Manager）的機密複製成 Kubernetes Secret，並依你設定的間隔保持同步。服務讀取這個 Secret 的方式和一般 Secret 相同。

這個元件是選用的。加上 SecretStore 之前，它不會做任何事。webhook 的憑證由 cert-manager 簽發，所以 Chart 自己的憑證控制器是關閉的。

Operator 會讀取、建立、更新和刪除每個 Namespace 的 Secret，也會替每個 Namespace 建立 ServiceAccount token，讓 SecretStore 能以 ServiceAccount 身分登入。

## 安裝前準備

- 先安裝 cert-manager，由它簽發 webhook 的憑證。
- 準備好機密庫，以及只能讀取的憑證。

## 使用方式

在服務所在的 Namespace 建立 SecretStore，再建立 ExternalSecret 指定要複製的值。詳見[從外部機密庫同步機密](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/external-secrets/)，裡面有 OpenBao、Vault、1Password 和 AWS Secrets Manager 的步驟。

## 移除

移除 Chart 時，CRD 也會刪除，所有 SecretStore 和 ExternalSecret 會跟著消失。在預設的 `creationPolicy: Owner` 下，ExternalSecret 擁有自己的 Secret，Secret 也會刪除，服務就讀不到值了。移除前，把必須保留 Secret 的 ExternalSecret 設成 `creationPolicy: Orphan`，或先複製這些 Secret。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### ExternalSecret 沒有就緒

執行 `kubectl -n <namespace> describe externalsecret <name>`，狀態會告訴你是不是 SecretStore 的問題。再執行 `kubectl -n <namespace> describe secretstore <name>`，確認它連得到機密庫。

### 建立 SecretStore 失敗

寫入 SecretStore 時，請求會經過 webhook。用 `kubectl -n external-secrets get pods` 和 `kubectl -n external-secrets get certificate` 檢查。
