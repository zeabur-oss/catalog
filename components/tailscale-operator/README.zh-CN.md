## 简介

Tailscale Kubernetes operator 会把 Service 或 Ingress 放到你的 tailnet 上，无需对互联网开放任何端口。Ingress 会得到一个 MagicDNS 名称和 Let's Encrypt 证书，所以没有域名的主机也能提供受信任的 HTTPS。Service 则原样转发，没有 TLS。

这个组件是可选的。它需要 Tailscale 账号，Ingress 还需要 tailnet 开启 HTTPS 和 MagicDNS。Operator 不代理 Kubernetes API，也不创建 impersonation 角色。

Operator 会创建、更新和删除每个 Namespace 的 Service 和 Ingress，读取节点和 EndpointSlice，并管理 ValidatingAdmissionPolicy。在自己的 `tailscale` Namespace，管理 Secret、ConfigMap、ServiceAccount、StatefulSet、Deployment、Role 和 RoleBinding。Operator 以特权运行，并安装八个 CRD 和 IngressClass `tailscale`。

## 安装前准备

创建一个 Tailscale OAuth client，把它的值填入输入项：

- `oauthClientId` 是 client ID，`oauthClientSecret` 是 client secret。
- client 需要 Services、Devices Core 和 Auth Keys 的写入范围，以及标签 `tag:k8s-operator`。
- 在 tailnet 策略里，`tag:k8s-operator` 要是 `tag:k8s` 的所有者，`tag:k8s` 是代理默认的标签。

## 使用方法

Zeabur 没有这个组件的界面。创建 class 为 `tailscale` 的 Ingress。下面的例子把 `shop` Namespace 里的 Service `web`（端口 8080）放到 tailnet 上：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web-tailnet
  namespace: shop
spec:
  ingressClassName: tailscale
  tls:
    - hosts:
        - web
  rules:
    - http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port:
                  number: 8080
```

Operator 处理完后，`kubectl -n shop get ingress web-tailnet` 的 `ADDRESS` 会显示 MagicDNS 名称。在 tailnet 内的任何设备用 `https://` 打开这个名称即可。要改用第 3 层暴露 Service，给它加上注解 `tailscale.com/expose: "true"`。详见[用 Ingress 把集群工作负载暴露到 tailnet](https://tailscale.com/kb/1439/kubernetes-operator-cluster-ingress)。

## 移除

移除 Operator 时，它的八个 CRD 和 IngressClass 也会被删除。先删除 class 为 `tailscale` 的 Ingress 和已暴露的 Service，之后检查 `tailscale` Namespace 有没有留下 StatefulSet 和 Secret。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### Ingress 没有地址

用 `kubectl -n tailscale logs deploy/operator` 查看 Operator 的日志。检查 OAuth client 的范围，以及 tailnet 策略里的 `tag:k8s-operator` 和 `tag:k8s`，并确认 tailnet 已开启 HTTPS 和 MagicDNS。
