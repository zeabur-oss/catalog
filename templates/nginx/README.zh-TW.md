## 簡介

[NGINX](https://nginx.org/) 是網頁伺服器，也能當反向代理。

這個範本在你的網域後面跑一個 NGINX。它對每個請求都只回一行純文字，不提供任何檔案。要改變它的行為，請編輯 ConfigMap 裡的設定。

## 開始使用

1. 在 Zeabur 開源版打開 `nginx` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `nginx.example.com`。
2. 頁面會顯示 `Hello from Nginx deployed on Zeabur!` 這行文字。不需要登入，範本也不會建立任何機密。
3. 要提供其他內容，請見「設定」。

## 設定

### 改變 NGINX 的行為

整份 `nginx.conf` 都在 ConfigMap `nginx` 裡。NGINX 只在啟動時讀取它。打開專案的**物件**頁面，打開 ConfigMap `nginx`，切換到 YAML 檢視，按**編輯**修改，檢查後套用。再按 `nginx` 服務頁上方的**重新啟動**。

根檔案系統是唯讀的。請保留 `listen 8080`、`pid` 那一行，以及指向 `/tmp` 的 `*_temp_path` 那幾行。詳見 NGINX 文件的 [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html)。

### 把請求轉給另一個服務

把 `location /` 裡的 `return` 那一行，換成指向同一個命名空間裡某個 Service 的 `proxy_pass`：

```nginx
location / {
  proxy_pass http://my-app:3000;
}
```

詳見 [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html) 的 "Setting Up a Simple Proxy Server"。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `nginx` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

詳見 Zeabur 開源版文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

### 編輯後新的 Pod 起不來

設定有錯時，NGINX 不會啟動。新的 Pod 就緒之前，舊的 Pod 會繼續提供服務，所以網站不會中斷。打開 `nginx` 服務的**日誌**分頁，用上方的選擇器選新的副本，看 NGINX 拒絕的那一行。在**物件**頁面修正 ConfigMap，再按一次**重新啟動**。
