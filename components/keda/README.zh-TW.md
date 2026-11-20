## 簡介

KEDA 依事件調整 Deployment、StatefulSet 等工作負載的副本數，事件可以是佇列長度、排程或某個指標，也可以縮到零。縮放規則寫在工作負載所在 Namespace 的 ScaledObject 裡。

單節點上，KEDA 是選用的：它佔用的記憶體比縮放省下的還多。多節點時，Operator 跑兩個副本，節點清空時縮放不會中斷。

KEDA 會讀取每個 Namespace 的縮放目標，以及觸發器用到的 Secret，並提供外部指標 API（`v1beta1.external.metrics.k8s.io`）。

## 使用方式

Zeabur 沒有 KEDA 的畫面。在工作負載旁建立 ScaledObject。下面的例子讓 `web` 在 08:00 到 20:00 之間跑 2 個副本，其餘時間為零：

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: web
  namespace: shop
spec:
  scaleTargetRef:
    name: web
  minReplicaCount: 0
  maxReplicaCount: 5
  triggers:
    - type: cron
      metadata:
        timezone: Asia/Taipei
        start: 0 8 * * *
        end: 0 20 * * *
        desiredReplicas: "2"
```

用 `kubectl -n shop get scaledobject web` 檢查。詳見[Scaling Deployments](https://keda.sh/docs/2.21/concepts/scaling-deployments/) 和 [Scalers 清單](https://keda.sh/docs/2.21/scalers/)。要依流量縮放網頁服務，再安裝 KEDA HTTP Add-on。

## 移除

CRD 屬於 Chart，所以移除 KEDA 會把它們刪掉，所有 ScaledObject、ScaledJob 和 TriggerAuthentication 也跟著消失。工作負載會保持當下的副本數，縮到零的服務就停在零。移除前先用 `kubectl get scaledobject -A` 檢查，移除後自己調整這些工作負載的副本數。KEDA HTTP Add-on 需要 KEDA，所以它還裝著時，Zeabur 會拒絕移除 KEDA。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### ScaledObject 沒有就緒

執行 `kubectl -n <namespace> get scaledobject`，看 `READY` 欄。再執行 `kubectl -n <namespace> describe scaledobject <name>`，並用 `kubectl -n keda logs deploy/keda-operator` 看 Operator 的日誌。
