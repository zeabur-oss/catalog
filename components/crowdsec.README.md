## Introduction

CrowdSec reads Traefik's access logs, spots brute-force attempts and scanners, and blocks their addresses before the request reaches a service. A Traefik plugin does the blocking on the `web` and `websecure` entry points, so it covers every app behind the ingress and no app opts in.

Install it when your cluster serves the internet through Traefik. It needs Traefik and a component that provides `certificates`. Installing it also changes Traefik: it adds the plugin, which Traefik downloads from plugins.traefik.io at start, and it keeps each client's address and user agent in the access log.

The component fails open. If the CrowdSec API is unreachable, Traefik keeps serving traffic. The online API is off, so no data leaves the cluster. The keys it needs are generated for you.

An agent on each node reads `/var/log` through a read-only host path and runs privileged. Its client certificate can send and read alerts and delete decisions.

## Use it

CrowdSec has no Zeabur screen. List what it blocks and what it saw:

```sh
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions list
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli alerts list
```

See the [cscli decisions reference](https://doc.crowdsec.net/docs/cscli/cscli_decisions) to add or delete decisions.

## Remove it

Removal deletes the agents, the local API and the bouncer middleware. The volume with the decisions stays in the `crowdsec` namespace. Check that the Traefik Pods are healthy after removal: `kubectl -n traefik get pods`. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A real visitor is blocked

Find the address in the decisions and delete it:

```sh
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions list
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions delete --ip <address>
```

### Nothing is ever blocked

Check that the agents run and that they read Traefik's log:

```sh
kubectl -n crowdsec get pods
kubectl -n crowdsec exec ds/crowdsec-agent -- cscli metrics
```
