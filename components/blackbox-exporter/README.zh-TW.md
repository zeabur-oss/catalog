## 簡介

Prometheus Blackbox Exporter 從叢集內部檢查網站或連接埠有沒有回應。附兩個模組。`http_2xx` 會跟著重新導向，要求最後得到 2xx 回應，先試 IPv6，再試 IPv4。`tcp_connect` 建立 TCP 連線。兩者都在 5 秒後放棄。

想對自己的目標做可用性檢查和警示時，就裝它。需要 VictoriaMetrics 來保存結果。多節點時跑兩個副本，所以只有一個副本探測失敗時，不一定是目標掛了。沒有 Kubernetes API 權限，只對外發出探測。

## 使用方式

Blackbox Exporter 沒有 Zeabur 開源版畫面。用 `VMProbe` 告訴指標代理程式要探測什麼。這個範例每分鐘檢查一個網站：

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMProbe
metadata:
  name: example-site
  namespace: observability
spec:
  jobName: example-site
  interval: 60s
  module: http_2xx
  vmProberSpec:
    url: blackbox-exporter-prometheus-blackbox-exporter.observability.svc:9115
  targets:
    static:
      targets:
        - https://example.com
```

目標有回應時，指標 `probe_success` 是 `1`。要依它發警示，請寫 `VMRule`。詳見 [VMProbe](https://docs.victoriametrics.com/operator/resources/vmprobe/) 和 [VMRule](https://docs.victoriametrics.com/operator/resources/vmrule/)。

## 移除

移除會刪除匯出器。你建立的 `VMProbe` 會留著，刪除之前，探測都會失敗。已保存的結果在 VictoriaMetrics 的保留期限到期前都會留著。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 探測一直失敗

手動執行同一個探測，並開啟除錯輸出：

```sh
kubectl -n observability port-forward svc/blackbox-exporter-prometheus-blackbox-exporter 9115:9115
curl 'localhost:9115/probe?module=http_2xx&target=https://example.com&debug=true'
```

輸出會列出每個步驟，例如 DNS、連線和重新導向。
