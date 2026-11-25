## 簡介

Kubernetes Metrics Server 提供 `metrics.k8s.io` API，內容是每個 Pod 和節點目前用了多少 CPU 與記憶體。每 15 秒向 kubelet 收集數字，不保存歷史，所以是取得即時用量最省資源的來源。

想看 Zeabur 開源版的即時用量、使用 `kubectl top` 或自動擴縮時，就裝它。這個 API 停擺時，Horizontal Pod Autoscaler 也跟著停止。多節點時跑兩個副本，並設定 PodDisruptionBudget，節點排空時仍有一個副本可用。

Metrics Server 讀取 Pod、節點和 Namespace，以及 kubelet 的節點指標，並註冊一個聚合 API。元件會加一條 NetworkPolicy，讓 API server 連到 10250 連接埠，並在 restricted 的 Pod Security 層級下執行。

## 使用方式

在 Zeabur 開源版，服務的**用量**分頁從這個 API 取得**目前用量**。沒裝時，分頁會出現**安裝**。詳見[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/#看用量)。

用 kubectl 確認：

```sh
kubectl top pods -A
```

## 移除

移除會刪除指標 API。`kubectl top`、Zeabur 開源版的即時用量和自動擴縮都會沒有資料。其他已安裝的元件（例如 Vertical Pod Autoscaler）還需要它時，Zeabur 開源版會拒絕移除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### kubectl top 說指標 API 無法使用

確認 API 可用，Pod 也已就緒：

```sh
kubectl get apiservice v1beta1.metrics.k8s.io
kubectl -n kube-system get pods -l app.kubernetes.io/name=metrics-server
```
