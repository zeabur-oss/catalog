## Introduction

VictoriaMetrics keeps the cluster's metrics history and evaluates alert rules. It scrapes every 30 seconds and keeps 7 days of data on a single node, or 14 days on several nodes, in a 5 Gi volume. Its Alertmanager sends Zeabur OSS's notifications: each project sets its own channels, and the cluster administrator sets the cluster's.

Install it when you want usage history and notifications in Zeabur OSS. It scrapes Pods in every namespace and reads builds, databases, backups and certificates in every namespace to raise alerts. Its node exporter runs on the host network and reads the host's `/proc` and `/sys`.

The component turns off what Zeabur OSS does not read: Grafana, the default dashboards and the API server metrics. Only Alertmanager's own senders can post alerts to it: the alert rules, the metrics agent and Falco. Falco alerts go to the cluster's channels only.

## Use it

In Zeabur OSS, a service's **Usage** tab shows the usage history from this store. See [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/#view-usage). Add channels and send a test under **Cluster** → **Notifications**. See [Send cluster alerts to notification channels](https://oss.zeabur.com/docs/zeabur/cluster/notifications/).

## Remove it

Removal deletes the metrics store, the agent, the alert rules, Alertmanager and the exporters. The history volume stays in the `observability` namespace. Delete it yourself when you no longer need the data.

Usage history and notifications stop after removal. Zeabur OSS refuses to remove it while another installed component, such as Blackbox Exporter, needs it. The VictoriaMetrics operator stays. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### History or notifications show no data

Check that the store and the agents are operational. Each command should print `operational`:

```sh
kubectl -n observability get vmsingle -o jsonpath='{.items[0].status.updateStatus}'
kubectl -n observability get vmagent -o jsonpath='{.items[0].status.updateStatus}'
```

If one does not, read the Pods with `kubectl -n observability get pods`, and check that the operator runs.
