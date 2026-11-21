## 简介

KEDA HTTP Add-on 按流量调整 Web 服务的副本数，空闲时可以缩到零。请求会经过拦截器代理（interceptor proxy）。服务没有副本时，拦截器会先扣住第一个请求，等副本就绪再转过去。它需要 KEDA 组件。

组件把 Pod 的安全上下文设为 `runAsNonRoot` 并套用默认的 seccomp 配置，符合 restricted 的 Pod Security 级别。单节点上，它至少运行一个拦截器和一个 scaler；多节点上，至少运行三个拦截器和两个 scaler。

Add-on 会读取每个 Namespace 的 Service、EndpointSlice 和它自己的资源，也能创建和修改 ScaledObject。Chart 仍会安装 `HTTPScaledObject` CRD，但上游已经弃用它，请改用 `InterceptorRoute`。

## 使用方法

Zeabur 开源版没有这个 Add-on 的界面。先创建 InterceptorRoute，再创建 ScaledObject。下面的例子伸缩 `shop` Namespace 里的 Service `web`（端口 8080）：

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

接着把 `app.example.com` 的流量送到 `keda` Namespace 的 Service `keda-add-ons-http-interceptor-proxy`（端口 8080），不要直接送到 `web`。详见 [Autoscale an App](https://keda.sh/http-add-on/0.16/user-guide/autoscale-an-app/) 和 [Configure Ingress](https://keda.sh/http-add-on/0.16/user-guide/configure-ingress/)。

## 移除

移除 Add-on 会删除它的 CRD，所有 InterceptorRoute 和 HTTPScaledObject 也随之消失。拦截器的 Service 也会被删除，所以要先把流量改回直接送到服务。带有 `external-push` 触发器的 ScaledObject 指向 Add-on 的 scaler，要删掉或改掉触发器。缩到零的服务会停在零，要自己把副本数调上来。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 服务一直不扩容

先创建 InterceptorRoute，再创建 ScaledObject。InterceptorRoute 还不存在时，KEDA 会退回使用 CPU 指标，就无法扩容。删掉 ScaledObject 再重新应用。用 `kubectl -n shop get scaledobject web` 检查。
