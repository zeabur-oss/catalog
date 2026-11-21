## Introduction

Falco watches system calls on every node and flags suspicious behavior in containers, such as a shell opened in a container or a privilege escalation. It uses a built-in eBPF probe, so it needs no kernel module. The rules update only when the chart does.

Install it when you want runtime alerts in Zeabur OSS. Falcosidekick forwards each alert to a `PolicyReport` in the namespace it concerns, and to a `ClusterPolicyReport` when it has no namespace. Each report keeps the newest 100 alerts. Alerts carry command lines and file paths, so only namespace admins can read their namespace's report. Bind the `falco-alert-reader` ClusterRole in a namespace to let others read it.

Falco runs privileged on every node. Falcosidekick reads Pod and node metadata and writes reports in every namespace. With VictoriaMetrics installed, alerts of priority warning and above also go to the cluster's notification channels, never to project channels.

## Before you install

Each node needs a kernel with BTF, which means Linux 5.8 or later. Check that `/sys/kernel/btf/vmlinux` exists on every node.

## Use it

In Zeabur OSS, a service's **Overview** lists its **Runtime alerts**. Alerts without a namespace appear under **Node alerts** on **Cluster** → **Overview**. See [Runtime alerts (Falco)](https://oss.zeabur.com/docs/zeabur/cluster/diagnosis/#runtime-alerts-falco). To send them to Slack or email, see [Send cluster alerts to notification channels](https://oss.zeabur.com/docs/zeabur/cluster/notifications/).

## Remove it

Removal deletes the Falco DaemonSet and falcosidekick, so detection stops. Alerts live in `PolicyReport` objects. Copy what you need first with `kubectl get policyreport -A -o yaml`. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A Falco Pod does not start

Read the Pods and the Falco log. A node without BTF fails to load the probe:

```sh
kubectl -n falco get pods
kubectl -n falco logs ds/falco -c falco
```

### Zeabur OSS shows no alerts

Check that falcosidekick runs and that its log has no write errors:

```sh
kubectl -n falco logs deploy/falco-falcosidekick
```
