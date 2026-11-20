## 簡介

VictoriaMetrics operator 依照 `VMSingle`、`VLSingle` 這類自訂資源，建立並管理指標和日誌的儲存服務。它也讀得懂 Prometheus operator 的物件，所以現有的應用程式資訊清單照樣能用。

不用單獨安裝。VictoriaMetrics 和 VictoriaLogs 需要這個 operator，Zeabur 會先裝好。operator 需要 cert-manager（或其他提供 `certificates` 的元件），來簽發 admission webhook 的憑證。

operator 管理每個 Namespace 的物件，能讀取所有 Secret、在任何 Namespace 建立 Pod，也能建立 ClusterRole 和綁定，所以權杖等於 cluster-admin。

## 使用方式

operator 沒有 Zeabur 畫面。安裝 VictoriaMetrics 或 VictoriaLogs 之後 operator 才開始工作。確認 operator 在執行，並列出它管理的物件：

```sh
kubectl -n observability get deploy victoria-metrics-operator
kubectl get vmsingle,vlsingle -A
```

要自己寫抓取目標、警示規則這類物件，詳見 [operator 文件](https://docs.victoriametrics.com/operator/)。

## 移除

移除前，清理 Job 會刪除所有 Namespace 裡的 VictoriaMetrics 和 VictoriaLogs 物件，包含儲存服務、代理程式、警示規則、抓取物件、探測，以及存成 `VMAlertmanagerConfig` 的通知管道。請先移除 VictoriaMetrics 和 VictoriaLogs，想沿用的規則或管道先自己備份。

CRD 會留著，清理 hook 的 ClusterRole 和 ClusterRoleBinding 也會留著。其他已安裝的元件還需要它時，Zeabur 會拒絕移除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 建立 VictoriaMetrics 物件時出現 webhook 錯誤

admission webhook 需要憑證。確認憑證已就緒，operator 的 Pod 也在執行：

```sh
kubectl -n observability get certificate victoria-metrics-operator-validation
kubectl -n observability get pods
```
