## 簡介

CrowdSec 讀取 Traefik 的存取日誌，找出暴力破解和掃描行為，在請求碰到服務之前封鎖對方的位址。封鎖由 Traefik 外掛在 `web` 和 `websecure` 入口完成，所以 Ingress 後面的每個應用程式都受保護，不用各自設定。

叢集透過 Traefik 對外提供服務時，就裝它。需要 Traefik 和提供 `certificates` 的元件。安裝時也會改動 Traefik：加上外掛（Traefik 啟動時從 plugins.traefik.io 下載），並在存取日誌保留每個用戶端的位址和 User-Agent。

這個元件失效時會放行。CrowdSec API 連不上時，Traefik 繼續處理流量。線上 API 是關閉的，所以沒有資料離開叢集。需要的金鑰會自動產生。

每個節點上的代理程式以特權模式執行，並唯讀掛載主機的 `/var/log`。代理程式的用戶端憑證能傳送和讀取警示，也能刪除封鎖決定。

## 使用方式

CrowdSec 沒有 Zeabur 開源版畫面。列出封鎖了什麼、看到了什麼：

```sh
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions list
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli alerts list
```

要新增或刪除封鎖決定，詳見 [cscli decisions 參考](https://doc.crowdsec.net/docs/cscli/cscli_decisions)。

## 移除

移除會刪除代理程式、本機 API 和 bouncer middleware。存放封鎖決定的磁碟區會留在 `crowdsec` Namespace。移除後，用 `kubectl -n traefik get pods` 確認 Traefik 的 Pod 正常。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 正常的訪客被封鎖

在封鎖決定裡找到位址，再刪除：

```sh
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions list
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions delete --ip <位址>
```

### 一直沒有封鎖任何位址

確認代理程式有在執行，也有讀到 Traefik 的日誌：

```sh
kubectl -n crowdsec get pods
kubectl -n crowdsec exec ds/crowdsec-agent -- cscli metrics
```
