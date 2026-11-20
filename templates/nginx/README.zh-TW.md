## 簡介

[NGINX](https://nginx.org/) 是網頁伺服器，也能當反向代理。

這個範本在你的網域後面跑一個 NGINX。它對每個請求都只回一行純文字，不提供任何檔案。要改變它的行為，請編輯 ConfigMap 裡的設定。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://nginx.example.com`。
2. 頁面會顯示 `Hello from Nginx deployed on Zeabur!` 這行文字。不需要登入，範本也不會建立任何機密。
3. 要提供其他內容，請見「設定」。

## 設定

### 改變 NGINX 的行為

完整的 `nginx.conf` 放在 ConfigMap `nginx` 裡。Pod 只在啟動時讀取它，所以編輯後要重新啟動 Deployment：

```sh
kubectl -n <namespace> edit configmap nginx
kubectl -n <namespace> rollout restart deployment/nginx
```

根檔案系統是唯讀的。請保留 `listen 8080`、`pid` 那一行，以及指向 `/tmp` 的 `*_temp_path` 各行。詳見 NGINX 文件的 [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html)。

### 把請求轉給另一個服務

把 `location /` 裡的 `return` 那一行，換成指向同一個命名空間裡某個 Service 的 `proxy_pass`：

```nginx
location / {
  proxy_pass http://my-app:3000;
}
```

詳見 [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html) 的 "Setting Up a Simple Proxy Server"。

## 疑難排解

### 編輯後新的 Pod 起不來

設定有錯時，NGINX 會拒絕啟動。新的 Pod 就緒之前，舊的 Pod 會繼續提供服務，所以網站不會中斷。請查看新 Pod 的記錄，找出 NGINX 拒絕的那一行，修正 ConfigMap 後再重新啟動 Deployment：

```sh
kubectl -n <namespace> get pods
kubectl -n <namespace> logs <Pod 名稱>
```
