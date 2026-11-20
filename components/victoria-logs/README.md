## Introduction

VictoriaLogs keeps and searches container logs. A collector on each node reads the container logs and sends them to one log store. The store keeps 14 days of logs, or less when the logs reach 8 GiB of its 10 Gi volume.

Install it when you want **History** in a service's logs, which shows logs of replicas that no longer exist. It needs Kubernetes 1.34 or later and the VictoriaMetrics operator.

The collector skips the `kube-system`, `kube-public`, `kube-node-lease` and `observability` namespaces. It stores namespace, Pod and container names, but not Pod labels. On control-plane nodes of K3s and RKE2 it also reads the kube-apiserver audit log. The collector runs privileged, with read access to pod metadata and node log files. Only the collector, the metrics agent and Zeabur Proxy can reach the store.

## Use it

In Zeabur, open a service's **Logs** tab and use **History**. A cluster administrator must also enable `proxy.enabled` in Zeabur's Helm chart. See [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/#history).

To search by hand, forward the store's port and query it. A NetworkPolicy does not stop `kubectl port-forward`:

```sh
kubectl -n observability port-forward svc/vlsingle-victoria-logs 9428:9428
curl localhost:9428/select/logsql/query -d 'query=kubernetes.pod_namespace:shop' -d 'limit=10'
```

See the [querying guide](https://docs.victoriametrics.com/victorialogs/querying/) for the query language.

## Remove it

Removal deletes the log store and the collector. The volume with the stored logs stays in the `observability` namespace. Delete it yourself when you no longer need the logs. **History** stops working after removal. The VictoriaMetrics operator stays. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### History is empty

Check that the store and the collector are operational. Each command should print `operational`:

```sh
kubectl -n observability get vlsingle victoria-logs -o jsonpath='{.status.updateStatus}'
kubectl -n observability get vlagent collector -o jsonpath='{.status.updateStatus}'
```

Logs from the four skipped namespaces never appear.
