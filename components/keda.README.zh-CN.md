## 简介

KEDA 按事件调整 Deployment、StatefulSet 等工作负载的副本数，事件可以是队列长度、定时或某个指标，也可以缩到零。伸缩规则写在工作负载所在 Namespace 的 ScaledObject 里。

单节点上，KEDA 是可选的：它占用的内存比伸缩省下的还多。多节点时，Operator 运行两个副本，节点排空时伸缩不会中断。

KEDA 会读取每个 Namespace 的伸缩目标，以及触发器用到的 Secret，并提供外部指标 API（`v1beta1.external.metrics.k8s.io`）。

## 使用方法

Zeabur 没有 KEDA 的界面。在工作负载旁创建 ScaledObject。下面的例子让 `web` 在 08:00 到 20:00 之间运行 2 个副本，其余时间为零：

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
        timezone: Asia/Shanghai
        start: 0 8 * * *
        end: 0 20 * * *
        desiredReplicas: "2"
```

用 `kubectl -n shop get scaledobject web` 检查。详见 [Scaling Deployments](https://keda.sh/docs/2.21/concepts/scaling-deployments/) 和 [Scalers 列表](https://keda.sh/docs/2.21/scalers/)。要按流量伸缩 Web 服务，再安装 KEDA HTTP Add-on。

## 移除

CRD 属于 Chart，所以移除 KEDA 会把它们删掉，所有 ScaledObject、ScaledJob 和 TriggerAuthentication 也随之消失。工作负载会保持当下的副本数，缩到零的服务就停在零。移除前先用 `kubectl get scaledobject -A` 检查，移除后自己调整这些工作负载的副本数。KEDA HTTP Add-on 需要 KEDA，所以它还装着时，Zeabur 会拒绝移除 KEDA。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### ScaledObject 没有就绪

执行 `kubectl -n <namespace> get scaledobject`，查看 `READY` 列。再执行 `kubectl -n <namespace> describe scaledobject <name>`，并用 `kubectl -n keda logs deploy/keda-operator` 查看 Operator 的日志。
