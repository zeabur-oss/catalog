## Introduction

The KEDA HTTP Add-on scales a web service on its traffic, and it can scale it down to zero when idle. Requests go through an interceptor proxy. When the service has no replica, the interceptor holds the first request until a replica is ready. It needs the KEDA component.

The component sets the pod security context to `runAsNonRoot` with the default seccomp profile, which the restricted Pod Security level requires. On a single node it runs at least one interceptor and one scaler. On several nodes it runs at least three interceptors and two scalers.

The add-on reads Services, EndpointSlices and its own resources in every namespace, and it can create and change ScaledObjects. The chart still installs the `HTTPScaledObject` CRD, but upstream has deprecated it. Use `InterceptorRoute`.

## Use it

Zeabur OSS has no screen for the add-on. Create an InterceptorRoute first, then a ScaledObject. This example scales the Service `web` (port 8080) of the `shop` namespace:

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

Then send traffic for `app.example.com` to the Service `keda-add-ons-http-interceptor-proxy` (port 8080) in the `keda` namespace, not straight to `web`. See [Autoscale an App](https://keda.sh/http-add-on/0.16/user-guide/autoscale-an-app/) and [Configure Ingress](https://keda.sh/http-add-on/0.16/user-guide/configure-ingress/).

## Remove it

Removing the add-on deletes its CRDs, and every InterceptorRoute and HTTPScaledObject with them. It also deletes the interceptor Service, so route traffic back to the services first. ScaledObjects with an `external-push` trigger point to the add-on's scaler, so delete them or change their triggers. A service at zero replicas stays at zero, so scale it up yourself. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### The service never scales up

Create the InterceptorRoute before the ScaledObject. If it does not exist yet, KEDA falls back to a CPU metric and scale-up does not work. Delete the ScaledObject and apply it again. Check it with `kubectl -n shop get scaledobject web`.
