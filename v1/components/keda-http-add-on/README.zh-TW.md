## 簡介

KEDA HTTP Add-on 依流量調整網頁服務的副本數，閒置時可以縮到零。請求會經過攔截器代理（interceptor proxy）。服務沒有副本時，攔截器會先扣住第一個請求，等副本就緒再轉過去。它需要 KEDA 元件。

元件把 Pod 設成 `runAsNonRoot` 並套用預設的 seccomp 設定檔，符合 restricted 的 Pod Security 等級。單節點上，它至少跑一個攔截器和一個 scaler；多節點上，至少跑三個攔截器和兩個 scaler。

Add-on 會讀取每個 Namespace 的 Service、EndpointSlice 和它自己的資源，也能建立和修改 ScaledObject。Chart 仍會安裝 `HTTPScaledObject` CRD，但上游已經棄用它，請改用 `InterceptorRoute`。

## 使用方式

Zeabur 開源版沒有這個 Add-on 的畫面。先建立 InterceptorRoute，再建立 ScaledObject。下面的例子縮放 `shop` Namespace 裡的 Service `web`（連接埠 8080）：

```yaml
apiVersion: http.keda.sh/v1beta1
kind: InterceptorRoute
metadata:
  name: web
  namespace: shop
spec:
  target:
    service: web
    port: 8080
  rules:
    - hosts:
        - app.example.com
  scalingMetric:
    concurrency:
      targetValue: 100
---
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: web
  namespace: shop
spec:
  scaleTargetRef:
    name: web
  minReplicaCount: 0
  maxReplicaCount: 10
  cooldownPeriod: 300
  triggers:
    - type: external-push
      metadata:
        scalerAddress: keda-add-ons-http-external-scaler.keda:9090
        interceptorRoute: web
```

接著把 `app.example.com` 的流量送到 `keda` Namespace 的 Service `keda-add-ons-http-interceptor-proxy`（連接埠 8080），不要直接送到 `web`。詳見 [Autoscale an App](https://keda.sh/http-add-on/0.16/user-guide/autoscale-an-app/) 和 [Configure Ingress](https://keda.sh/http-add-on/0.16/user-guide/configure-ingress/)。

## 移除

移除 Add-on 會刪除它的 CRD，所有 InterceptorRoute 和 HTTPScaledObject 也跟著消失。攔截器的 Service 也會被刪除，所以要先把流量改回直接送到服務。帶有 `external-push` 觸發器的 ScaledObject 指向 Add-on 的 scaler，要刪掉或改掉觸發器。縮到零的服務會停在零，要自己把副本數調上來。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 服務一直不擴增

先建立 InterceptorRoute，再建立 ScaledObject。InterceptorRoute 還不存在時，KEDA 會退回用 CPU 指標，擴增就不會運作。刪掉 ScaledObject 再重新套用。用 `kubectl -n shop get scaledobject web` 檢查。
