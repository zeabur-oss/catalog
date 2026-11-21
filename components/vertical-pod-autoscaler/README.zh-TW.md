## 簡介

Vertical Pod Autoscaler（VPA）的 recommender 會觀察工作負載實際用了多少 CPU 和記憶體，並建議資源保留量。這個元件只安裝 recommender。updater 和 admission controller 都是關閉的，所以不會驅逐或改寫任何 Pod。建議在你套用之前不會改變任何東西。

想依實際用量調整保留量時，就裝它。VPA 需要 Metrics Server，Zeabur 開源版會先裝好。recommender 讀取每個 Namespace 的 Pod、工作負載和指標，並寫入 `VerticalPodAutoscaler` 物件的狀態和檢查點，並在 restricted 的 Pod Security 層級下執行。

## 使用方式

VPA 沒有 Zeabur 開源版畫面。替工作負載建立 `VerticalPodAutoscaler`，並設定 `updateMode: "Off"`：

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: web
  namespace: shop
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
  updatePolicy:
    updateMode: "Off"
```

recommender 有資料之後，讀取建議：

```sh
kubectl -n shop get vpa web -o jsonpath='{.status.recommendation.containerRecommendations}'
```

自己把數值複製到工作負載的保留量。詳見 [VPA 文件](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)。

## 移除

移除會刪除 recommender。CRD 會留著，你的 `VerticalPodAutoscaler` 物件也會留著，但不再更新。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### VerticalPodAutoscaler 沒有建議

看它的 conditions，再看 recommender 的日誌：

```sh
kubectl -n shop describe vpa web
kubectl -n vpa-system logs deploy/vpa-vertical-pod-autoscaler-recommender
```

出現 `NoPodsMatched` 表示 `targetRef` 沒有選到任何 Pod，檢查名稱和 kind。
