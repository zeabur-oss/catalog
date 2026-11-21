## 簡介

VictoriaMetrics 保存叢集的指標歷史，並評估警示規則。每 30 秒抓取一次指標，單節點保留 7 天，多節點保留 14 天，資料放在 5 Gi 的磁碟區。Alertmanager 負責送出 Zeabur 開源版的通知：每個專案設定自己的管道，叢集管理員設定叢集的管道。

想在 Zeabur 開源版看用量歷史、收通知時就裝它。元件會抓取每個 Namespace 的 Pod，也會讀取每個 Namespace 的建置、資料庫、備份和憑證來產生警示，節點匯出器跑在主機網路上，讀取主機的 `/proc` 和 `/sys`。

這個元件關掉了 Zeabur 開源版用不到的部分：Grafana、預設儀表板和 API server 指標。能把警示送進 Alertmanager 的只有警示規則、指標代理程式和 Falco。Falco 的警示只送到叢集的管道。

## 使用方式

在 Zeabur 開源版，服務的**用量**分頁會顯示 VictoriaMetrics 保存的用量歷史。詳見[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/#看用量)。到**叢集** → **通知**新增管道並傳送測試，詳見[把叢集警示送到通知管道](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/notifications/)。

## 移除

移除會刪除指標儲存、代理程式、警示規則、Alertmanager 和匯出器。歷史資料的磁碟區會留在 `observability` Namespace，不再需要資料時自己刪除。

移除後，用量歷史和通知都會停止。其他已安裝的元件（例如 Blackbox Exporter）還需要它時，Zeabur 開源版會拒絕移除。VictoriaMetrics operator 會留著。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 歷史或通知沒有資料

確認儲存和代理程式的狀態是 operational。兩個指令都應該印出 `operational`：

```sh
kubectl -n observability get vmsingle -o jsonpath='{.items[0].status.updateStatus}'
kubectl -n observability get vmagent -o jsonpath='{.items[0].status.updateStatus}'
```

不是的話，用 `kubectl -n observability get pods` 看 Pod，並確認 operator 有在執行。
