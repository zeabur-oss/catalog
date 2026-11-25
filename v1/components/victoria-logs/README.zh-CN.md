## 简介

VictoriaLogs 保存并搜索容器日志。每个节点上的采集器读取容器日志，发送到同一个日志存储服务。日志保留 14 天，累积到 10 Gi 卷的 8 GiB 时，保留的天数会更少。

想在服务的日志中看到**历史记录**（已经不存在的副本的日志）时，就安装它。需要 Kubernetes 1.34 以上，以及 VictoriaMetrics operator。

采集器不收 `kube-system`、`kube-public`、`kube-node-lease` 和 `observability` 这四个 Namespace，只存 Namespace、Pod 和容器名称，不存 Pod 标签。在 K3s 和 RKE2 的控制平面节点上，也会读 kube-apiserver 的审计日志。采集器以特权模式运行，能读取 Pod 的元数据和节点上的日志文件。只有采集器、指标代理和 Zeabur Proxy 能访问存储服务。

## 使用方法

在 Zeabur 开源版打开服务的**日志**标签页，使用**历史记录**。集群管理员还要在 Zeabur 开源版的 Helm chart 中开启 `proxy.enabled`。详见[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/#历史记录)。

要自己搜索，先转发存储服务的端口再查询。NetworkPolicy 不会拦截 `kubectl port-forward`：

```sh
kubectl -n observability port-forward svc/vlsingle-victoria-logs 9428:9428
curl localhost:9428/select/logsql/query -d 'query=kubernetes.pod_namespace:shop' -d 'limit=10'
```

查询语法详见[查询指南](https://docs.victoriametrics.com/victorialogs/querying/)。

## 移除

移除会删除日志存储服务和采集器。存放日志的卷会留在 `observability` Namespace，不再需要日志时自己删除。移除后**历史记录**无法使用。VictoriaMetrics operator 会保留。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 历史记录是空的

确认存储服务和采集器的状态是 operational。两条命令都应该输出 `operational`：

```sh
kubectl -n observability get vlsingle victoria-logs -o jsonpath='{.status.updateStatus}'
kubectl -n observability get vlagent collector -o jsonpath='{.status.updateStatus}'
```

上面那四个 Namespace 的日志不会出现。
