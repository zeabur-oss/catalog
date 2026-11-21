## 簡介

[Uptime Kuma](https://github.com/louislam/uptime-kuma) 監看你的服務，掛掉時通知你，也能發布狀態頁面。

這個範本在你的網域後面跑一個 Uptime Kuma，資料存在磁碟區裡。這裡沒有自動產生的密碼，也沒有機密。

## 開始使用

1. 在 Zeabur 開源版打開 `uptime-kuma` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，也就是你在安裝表單填的網域。
2. 部署完成後馬上開啟這個網域。第一個進來的人會看到設定頁面並建立擁有者帳號。請搶在別人之前完成，因為誰先進來誰就是擁有者。
3. 新增你的第一個監控項目。

## 設定

### 監控項目掛掉時收到通知

在設定裡新增通知，再套用到你的監控項目。Uptime Kuma 支援許多通知服務，Apprise 還能再多接一批。詳見 [Notification Methods](https://uptime.kuma.pet/docs/Notification-Methods)。

### 發布狀態頁面

狀態頁面是給一般使用者看的。它會把結果快取 5 分鐘，所以更新比儀表板慢。詳見 [Status Page](https://uptime.kuma.pet/docs/Status-Page)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `uptime-kuma` 服務，確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本一直重新啟動時，打開**部署紀錄**，切到**事件**。如果事件寫著 `OOMKilled`，到**設定** → **運算資源**調高**記憶體上限**。詳見[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

網域或憑證一直停在等待中時，看[加上公開網域與 HTTPS](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)的「遇到問題」。

### Ping 監控失敗

Ping 監控用的是不需特權的 ICMP socket，容器沒有 `NET_RAW` 權限。Ping 能不能用，要看叢集網路怎麼處理 ICMP。Ping 不通的地方，改用 HTTP、TCP 或 DNS 監控。

### 忘記密碼

按服務頁上方的**終端機**，執行重設指令，再照提示操作：

```sh
npm run reset-password
```

不用 Zeabur 開源版時，用 kubectl 執行：

```sh
kubectl -n <namespace> exec -it uptime-kuma-0 -- npm run reset-password
```

詳見[Reset Password via CLI](https://uptime.kuma.pet/docs/Reset-Password-via-CLI)。
