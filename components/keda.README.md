## Introduction

KEDA scales Deployments, StatefulSets and other workloads on events, such as a queue length, a schedule or a metric, and it can scale them down to zero. You describe the scaling in a ScaledObject in the workload's namespace.

On a single node, KEDA is optional: it uses more memory than scaling saves. On several nodes, the operator runs two replicas, so a node drain does not stop scaling.

KEDA reads scale targets and the Secrets that triggers use in every namespace, and it serves the external metrics API (`v1beta1.external.metrics.k8s.io`).

## Use it

Zeabur has no screen for KEDA. Create a ScaledObject next to the workload. This example runs 2 replicas of `web` from 08:00 to 20:00 and none outside those hours:

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

Check it with `kubectl -n shop get scaledobject web`. See [Scaling Deployments](https://keda.sh/docs/2.21/concepts/scaling-deployments/) and the [list of scalers](https://keda.sh/docs/2.21/scalers/). To scale web services on traffic, add the KEDA HTTP Add-on.

## Remove it

The CRDs are part of the chart, so removing KEDA deletes them, and every ScaledObject, ScaledJob and TriggerAuthentication with them. A workload keeps the replica count it has at that moment. A service scaled to zero stays at zero. Before you remove KEDA, check `kubectl get scaledobject -A` and scale those workloads yourself afterwards. KEDA HTTP Add-on needs KEDA, so Zeabur refuses to remove KEDA while it is installed. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A ScaledObject is not ready

Run `kubectl -n <namespace> get scaledobject` and check the `READY` column. Then run `kubectl -n <namespace> describe scaledobject <name>` and read the operator's logs with `kubectl -n keda logs deploy/keda-operator`.
