## 简介

Vertical Pod Autoscaler（VPA）的 recommender 会观察工作负载实际用了多少 CPU 和内存，并建议资源预留量。这个组件只安装 recommender。updater 和 admission controller 都是关闭的，所以不会驱逐或改写任何 Pod。建议在你应用之前不会改变任何东西。

想根据实际用量调整预留量时，就安装它。VPA 需要 Metrics Server，Zeabur 开源版会先装好。recommender 读取每个 Namespace 的 Pod、工作负载和指标，并写入 `VerticalPodAutoscaler` 对象的状态和检查点，并在 restricted 的 Pod Security 级别下运行。

## 使用方法

VPA 没有 Zeabur 开源版界面。为工作负载创建 `VerticalPodAutoscaler`，并设置 `updateMode: "Off"`：

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

recommender 有数据之后，读取建议：

```sh
kubectl -n shop get vpa web -o jsonpath='{.status.recommendation.containerRecommendations}'
```

自己把数值复制到工作负载的预留量。详见 [VPA 文档](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)。

## 移除

移除会删除 recommender。CRD 会保留，你的 `VerticalPodAutoscaler` 对象也会保留，但不再更新。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### VerticalPodAutoscaler 没有建议

查看它的 conditions，再查看 recommender 的日志：

```sh
kubectl -n shop describe vpa web
kubectl -n vpa-system logs deploy/vpa-vertical-pod-autoscaler-recommender
```

出现 `NoPodsMatched` 表示 `targetRef` 没有选中任何 Pod，检查名称和 kind。
