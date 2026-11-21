## 簡介

錯誤頁面提供維護頁面，以及狀態碼 404、502、503、504 的頁面。它是一個小型的 [nginx](https://nginx.org/) 伺服器，頁面放在 ConfigMap 裡。

這個範本沒有網域，也不用登入。Zeabur 開源版會把專案網域的維護頁面和錯誤頁面導向這個服務。同一個命名空間的其他工作負載也能使用。

## 開始使用

1. 在 Zeabur 開源版打開 `error-pages` 服務。**概覽**顯示副本已就緒，服務就啟動好了。它沒有公開網域，**網路**分頁會顯示它的私有位址。
2. 其他工作負載透過 Service `error-pages` 連線。連接埠 8080 在 `http://error-pages:8080/<狀態碼>.html` 提供各個頁面，例如 `http://error-pages:8080/502.html`。連接埠 8081 對每個請求都回傳維護頁面和狀態碼 `503`。
3. 想從自己的電腦看頁面，先開一條通道再請求。Zeabur 開源版沒有對應的畫面，請用 kubectl：

   ```sh
   kubectl -n <namespace> port-forward service/error-pages 8080:8080
   curl http://localhost:8080/502.html
   ```

這個範本沒有需要讀取的機密。

## 設定

### 在 Traefik 的 errors 中介軟體使用這些頁面

在同一個命名空間建立 `Middleware`，把錯誤回應送到連接埠 8080，並依狀態碼取得對應的頁面：

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: error-pages
spec:
  errors:
    status:
      - "404"
      - "502-504"
    query: /{status}.html
    service:
      name: error-pages
      port: 8080
```

詳見 Traefik 文件的 [Errors](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/errorpages/)。

### 修改頁面文字

頁面是 ConfigMap `error-pages` 的 `404.html`、`502.html`、`503.html`、`504.html` 和 `maintenance.html`。在專案的**物件**頁面打開 ConfigMap `error-pages`，切到 YAML 檢視，按**編輯**，修改頁面後檢查並套用。Kubernetes 大約一分鐘內會更新掛載的檔案，nginx 每次請求都會重新讀取頁面，所以不用重新啟動任何東西。
