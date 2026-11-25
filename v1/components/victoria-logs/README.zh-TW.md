## 簡介

VictoriaLogs 保存並搜尋容器日誌。每個節點上的收集器讀取容器日誌，送到同一個日誌儲存服務。日誌保留 14 天，累積到 10 Gi 磁碟區的 8 GiB 時，保留的天數會更少。

想在服務的日誌看到**歷史紀錄**（已經不存在的副本的日誌）時，就裝它。需要 Kubernetes 1.34 以上，以及 VictoriaMetrics operator。

收集器不收 `kube-system`、`kube-public`、`kube-node-lease` 和 `observability` 這四個 Namespace，只存 Namespace、Pod 和容器名稱，不存 Pod 標籤。在 K3s 和 RKE2 的控制平面節點上，也會讀 kube-apiserver 的稽核日誌。收集器以特權模式執行，能讀取 Pod 的中繼資料和節點上的日誌檔。只有收集器、指標代理程式和 Zeabur Proxy 連得到儲存服務。

## 使用方式

在 Zeabur 開源版打開服務的**日誌**分頁，使用**歷史紀錄**。叢集管理員還要在 Zeabur 開源版的 Helm chart 開啟 `proxy.enabled`。詳見[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/#歷史紀錄)。

要自己搜尋，先轉發儲存服務的連接埠再查詢。NetworkPolicy 不會擋 `kubectl port-forward`：

```sh
kubectl -n observability port-forward svc/vlsingle-victoria-logs 9428:9428
curl localhost:9428/select/logsql/query -d 'query=kubernetes.pod_namespace:shop' -d 'limit=10'
```

查詢語法詳見[查詢指南](https://docs.victoriametrics.com/victorialogs/querying/)。

## 移除

移除會刪除日誌儲存服務和收集器。存放日誌的磁碟區會留在 `observability` Namespace，不再需要日誌時自己刪除。移除後**歷史紀錄**無法使用。VictoriaMetrics operator 會留著。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 歷史紀錄是空的

確認儲存服務和收集器的狀態是 operational。兩個指令都應該印出 `operational`：

```sh
kubectl -n observability get vlsingle victoria-logs -o jsonpath='{.status.updateStatus}'
kubectl -n observability get vlagent collector -o jsonpath='{.status.updateStatus}'
```

上面那四個 Namespace 的日誌不會出現。
