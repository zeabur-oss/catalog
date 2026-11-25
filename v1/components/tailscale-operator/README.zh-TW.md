## 簡介

Tailscale Kubernetes operator 會把 Service 或 Ingress 放上你的 tailnet，不必對網際網路開放任何連接埠。Ingress 會得到一個 MagicDNS 名稱和 Let's Encrypt 憑證，所以沒有網域的主機也能提供受信任的 HTTPS。Service 則原樣轉送，沒有 TLS。

這個元件是選用的。它需要 Tailscale 帳號，Ingress 還需要 tailnet 開啟 HTTPS 和 MagicDNS。Operator 不代理 Kubernetes API，也不建立 impersonation 角色。

Operator 會建立、更新和刪除每個 Namespace 的 Service 和 Ingress，讀取節點和 EndpointSlice，並管理 ValidatingAdmissionPolicy。在自己的 `tailscale` Namespace，管理 Secret、ConfigMap、ServiceAccount、StatefulSet、Deployment、Role 和 RoleBinding。Operator 以特權執行，並安裝八個 CRD 和 IngressClass `tailscale`。

## 安裝前準備

建立一個 Tailscale OAuth client，把它的值填進輸入項：

- `oauthClientId` 是 client ID，`oauthClientSecret` 是 client secret。
- client 需要 Services、Devices Core 和 Auth Keys 的寫入範圍，以及標籤 `tag:k8s-operator`。
- 在 tailnet 政策裡，`tag:k8s-operator` 要是 `tag:k8s` 的擁有者，`tag:k8s` 是代理預設的標籤。

## 使用方式

Zeabur 開源版沒有這個元件的畫面。建立 class 為 `tailscale` 的 Ingress。下面的例子把 `shop` Namespace 裡的 Service `web`（連接埠 8080）放上 tailnet：

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

Operator 處理完後，`kubectl -n shop get ingress web-tailnet` 的 `ADDRESS` 會顯示 MagicDNS 名稱。在 tailnet 內的任何裝置用 `https://` 開啟這個名稱即可。要改用第 3 層暴露 Service，替它加上註解 `tailscale.com/expose: "true"`。詳見[用 Ingress 把叢集工作負載暴露到 tailnet](https://tailscale.com/kb/1439/kubernetes-operator-cluster-ingress)。

## 移除

移除 Operator 時，它的八個 CRD 和 IngressClass 也會被刪除。先刪除 class 為 `tailscale` 的 Ingress 和已暴露的 Service，之後檢查 `tailscale` Namespace 有沒有留下 StatefulSet 和 Secret。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### Ingress 沒有位址

用 `kubectl -n tailscale logs deploy/operator` 看 Operator 的日誌。檢查 OAuth client 的範圍，以及 tailnet 政策裡的 `tag:k8s-operator` 和 `tag:k8s`，並確認 tailnet 已開啟 HTTPS 和 MagicDNS。
