## Introduction

The Vertical Pod Autoscaler (VPA) recommender watches how much CPU and memory a workload really uses and suggests resource requests. This component installs the recommender only. The updater and the admission controller are off, so nothing evicts or rewrites a Pod. A suggestion changes nothing until you apply it.

Install it when you want to size requests from real use. It needs Metrics Server, and Zeabur installs that first. It reads Pods, workloads and metrics in every namespace, and writes the status and checkpoints of `VerticalPodAutoscaler` objects. It runs under the restricted Pod Security level.

## Use it

VPA has no Zeabur screen. Create a `VerticalPodAutoscaler` for a workload, with `updateMode: "Off"`:

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

Read the suggestion once the recommender has data:

```sh
kubectl -n shop get vpa web -o jsonpath='{.status.recommendation.containerRecommendations}'
```

Copy the values into the workload's requests yourself. See the [VPA documentation](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).

## Remove it

Removal deletes the recommender. The CRDs stay, and so do your `VerticalPodAutoscaler` objects, which no longer get updates. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A VerticalPodAutoscaler has no recommendation

Read its conditions, then the recommender log:

```sh
kubectl -n shop describe vpa web
kubectl -n vpa-system logs deploy/vpa-vertical-pod-autoscaler-recommender
```

A `NoPodsMatched` condition means the `targetRef` selects no Pods. Check the name and the kind.
