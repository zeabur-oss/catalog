## Introduction

The Tailscale Kubernetes operator puts a Service or an Ingress on your tailnet. Nothing opens a port to the internet. An Ingress gets a MagicDNS name with a Let's Encrypt certificate, so a host without a domain still serves trusted HTTPS. A Service is proxied as it is, without TLS.

The component is optional. It needs a Tailscale account, and an Ingress needs HTTPS and MagicDNS enabled on the tailnet. The operator does not proxy the Kubernetes API and creates no impersonation role.

The operator creates, updates and deletes Services and Ingresses in every namespace, reads nodes and EndpointSlices, and manages ValidatingAdmissionPolicies. In its own `tailscale` namespace it manages Secrets, ConfigMaps, ServiceAccounts, StatefulSets, Deployments, Roles and RoleBindings. It runs privileged and installs eight CRDs and the IngressClass `tailscale`.

## Before you install

Create a Tailscale OAuth client and give its values to the inputs:

- `oauthClientId` is the client ID, and `oauthClientSecret` is its secret.
- The client needs write scope for Services, Devices Core and Auth Keys, and the tag `tag:k8s-operator`.
- In the tailnet policy, `tag:k8s-operator` must own `tag:k8s`, the default tag of the proxies.

## Use it

Zeabur has no screen for it. Create an Ingress with the class `tailscale`. This example puts the Service `web` (port 8080) of the `shop` namespace on the tailnet:

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

When the operator is done, `kubectl -n shop get ingress web-tailnet` shows the MagicDNS name in `ADDRESS`. Open that name with `https://` from any device on the tailnet. To expose a Service at layer 3 instead, annotate it with `tailscale.com/expose: "true"`. See [Expose cluster workloads to your tailnet with Ingress](https://tailscale.com/kb/1439/kubernetes-operator-cluster-ingress).

## Remove it

Removing the operator also deletes its eight CRDs and the IngressClass. Delete the Ingresses with class `tailscale` and the exposed Services first. Afterwards, check the `tailscale` namespace for leftover StatefulSets and Secrets. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### An Ingress gets no address

Read the operator's logs with `kubectl -n tailscale logs deploy/operator`. Check the OAuth client's scopes and the tailnet policy for `tag:k8s-operator` and `tag:k8s`. Check that HTTPS and MagicDNS are on for the tailnet.
