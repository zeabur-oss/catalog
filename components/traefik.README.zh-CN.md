## 简介

Traefik Proxy 接收来自互联网的请求，再按域名转给你的服务。Zeabur 的公开域名、重定向和密码都靠它，cert-manager 也需要集群里有入口组件。

Traefik 处理 Kubernetes Ingress 和 Gateway API 路由（GatewayClass `traefik`），也读取 Middleware 这类 Traefik 对象。IngressClass 是集群默认值。Service 会保留客户端地址（`externalTrafficPolicy: Local`），所以只有运行 Traefik 的节点会收到流量。多节点集群里，Traefik 每个节点运行一个 Pod，包含控制平面节点。

组件刻意关掉了几项功能：仪表盘、由 Chart 创建 Gateway，以及 Traefik 对象引用其他 Namespace 的对象。JSON 访问日志也只保留几个字段，不记录请求头和查询字符串。

Traefik 会读取每个 Namespace 的 Service、Secret 和路由，并写入路由的状态。

K3s 和 RKE2 自带 Traefik，Zeabur 会标示为由发行版安装。要改它的设置，见[修改内置组件的设置](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#修改内置组件的设置)。

## 使用方法

在 Zeabur 的服务**网络**标签页添加公开域名，Zeabur 会替你创建 Ingress 或路由。详见[添加公开域名与 HTTPS](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/)。

## 移除

移除 Traefik 后，所有入口流量都会中断。Ingress、HTTPRoute 和 Middleware 对象会保留，但没有东西处理，CRD 也会保留。cert-manager 需要入口组件，所以 cert-manager 还装着时，Zeabur 会拒绝移除 Traefik。移除前先确认没有公开域名还要用它。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 所有域名都没有响应

执行 `kubectl -n traefik get pods` 和 `kubectl -n traefik get service traefik`。Service 要从负载均衡器拿到外部地址，而且要有 Traefik Pod 运行在会收到流量的节点上。

### GatewayClass 没有被接受

执行 `kubectl describe gatewayclass traefik`，查看条件里的原因。
