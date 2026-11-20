## Introduction

Traefik Proxy receives requests from the internet and sends them to your services by domain. Zeabur public domains, redirects and passwords use it, and cert-manager needs an ingress component in the cluster.

It serves Kubernetes Ingress objects and Gateway API routes (GatewayClass `traefik`), and it reads Traefik objects such as Middleware. Its IngressClass is the cluster default. Its Service keeps client addresses (`externalTrafficPolicy: Local`), so only nodes that run Traefik receive traffic. On a multi-node cluster, Traefik runs one pod per node, control-plane nodes included.

The component turns some things off. The dashboard is off. The chart creates no Gateway. Traefik objects cannot reference objects in other namespaces. The JSON access log keeps a short list of fields and never logs headers or query strings.

Traefik reads Services, Secrets and routes in every namespace, and writes the status of routes.

K3s and RKE2 ship their own Traefik. Zeabur shows it as installed by the distribution. To change its settings, see [Change settings of built-in components](https://oss.zeabur.com/docs/zeabur/cluster/components/#change-settings-of-built-in-components).

## Use it

In Zeabur, add a public domain on the service's **Networking** tab. Zeabur creates the Ingress or route for you. See [Add public domains and HTTPS](https://oss.zeabur.com/docs/zeabur/configure/domains/).

## Remove it

Removing Traefik stops all ingress. Ingress, HTTPRoute and Middleware objects stay, but nothing serves them, and the CRDs stay. cert-manager needs an ingress component, so Zeabur refuses to remove Traefik while cert-manager is installed. Check that no public domain still needs it. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### No domain answers

Run `kubectl -n traefik get pods` and `kubectl -n traefik get service traefik`. The Service needs an external address from the load balancer, and a Traefik pod must run on a node that receives traffic.

### The GatewayClass is not accepted

Run `kubectl describe gatewayclass traefik` and read the condition's reason.
