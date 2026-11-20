## 簡介

cert-manager 會向簽發者申請 TLS 憑證，並在到期前續期。叢集要讓 Zeabur 的公開網域使用 HTTPS，就需要它。

這個元件會建立兩個 Let's Encrypt 的 ClusterIssuer：`letsencrypt` 和 `letsencrypt-staging`。兩者都透過 Traefik 做 HTTP-01 驗證，也都沒有設定電子郵件。建議先試 `letsencrypt-staging`：頻率限制寬鬆得多，但瀏覽器不信任它簽發的憑證。元件也開啟了 Gateway API 支援，cert-manager 可以替 Gateway 的 listener 簽發憑證，刪除 Certificate 時也會一併刪除對應的 TLS Secret。

cert-manager 會讀寫每個 Namespace 的 Secret，也會更新 webhook 的 CA bundle。

## 安裝前準備

- 先安裝 Traefik Proxy 這類入口元件。cert-manager 需要它的 Gateway API CRD。
- 做 HTTP-01 驗證時，網域的 DNS 記錄要指向叢集，且網際網路要能連到叢集的 80 連接埠。簽發者也要能對外連到 Let's Encrypt。
- 萬用字元網域（例如 `*.example.com`）需要 DNS-01 簽發者，要自己建立，做法見下方的頁面。

## 使用方式

在 Zeabur 的服務**網路**分頁加上公開網域。叢集有多個簽發者時，表單會出現**簽發者**欄位。詳見[加上公開網域與 HTTPS](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/)。同一頁也說明如何為萬用字元網域建立 DNS-01 簽發者。

## 移除

移除 cert-manager 後，就不會再簽發或續期。既有的 Certificate、TLS Secret 和 CRD 都會留著，但沒人續期，憑證到期就失效。兩個 ClusterIssuer 會被刪除。帳號金鑰 Secret `letsencrypt-account-key` 和 `letsencrypt-staging-account-key` 會留在 `cert-manager` Namespace。

Zeabur Builder 和 External Secrets Operator 需要它，只要其中一個還裝著，Zeabur 就會拒絕移除 cert-manager。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 憑證一直沒有就緒

執行 `kubectl get certificate -A`，再執行 `kubectl -n <namespace> describe certificate <name>`。接著用 `kubectl get challenges -A` 檢查驗證挑戰。HTTP-01 要求網域能從網際網路連到叢集的 80 連接埠。詳見 [cert-manager 的 ACME 疑難排解](https://cert-manager.io/docs/troubleshooting/acme/)。

### ClusterIssuer 沒有就緒

執行 `kubectl get clusterissuer` 和 `kubectl describe clusterissuer letsencrypt`。ACME 帳號註冊成功，簽發者才會就緒，這需要叢集能對外連到 Let's Encrypt。
