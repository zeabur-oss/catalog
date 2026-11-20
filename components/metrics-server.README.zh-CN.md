## 简介

Kubernetes Metrics Server 提供 `metrics.k8s.io` API，内容是每个 Pod 和节点当前用了多少 CPU 和内存。每 15 秒向 kubelet 收集数据，不保存历史，所以是获取实时用量最省资源的来源。

想看 Zeabur 的实时用量、使用 `kubectl top` 或自动扩缩时，就安装它。这个 API 停止时，Horizontal Pod Autoscaler 也会停止。多节点时运行两个副本，并设置 PodDisruptionBudget，节点排空时仍有一个副本可用。

Metrics Server 读取 Pod、节点和 Namespace，以及 kubelet 的节点指标，并注册一个聚合 API。组件会添加一条 NetworkPolicy，让 API server 连到 10250 端口，并在 restricted 的 Pod Security 级别下运行。

## 使用方法

在 Zeabur 中，服务的**用量**标签页从这个 API 获取**当前用量**。没安装时，标签页会显示**安装**。详见[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/#查看用量)。

用 kubectl 确认：

```sh
kubectl top pods -A
```

## 移除

移除会删除指标 API。`kubectl top`、Zeabur 的实时用量和自动扩缩都会没有数据。其他已安装的组件（例如 Vertical Pod Autoscaler）还需要它时，Zeabur 会拒绝移除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### kubectl top 提示指标 API 不可用

确认 API 可用，Pod 也已就绪：

```sh
kubectl get apiservice v1beta1.metrics.k8s.io
kubectl -n kube-system get pods -l app.kubernetes.io/name=metrics-server
```
