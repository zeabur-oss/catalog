## 简介

Falco 监控每个节点上的系统调用，找出容器内的可疑行为，例如打开 shell 或提升权限。它使用内置的 eBPF 探针，不需要内核模块。规则只在 chart 升级时更新。

想在 Zeabur 看到运行时告警时，就安装它。Falcosidekick 会把每条告警发到相关 Namespace 的 `PolicyReport`；没有 Namespace 的告警，则发到 `ClusterPolicyReport`。每份报告只保留最新的 100 条。告警带有命令行和文件路径，所以只有 Namespace 管理员能读取自己 Namespace 的报告。要让其他人也能读，在该 Namespace 绑定 ClusterRole `falco-alert-reader`。

Falco 在每个节点以特权模式运行。Falcosidekick 读取 Pod 和节点的元数据，并在每个 Namespace 写入报告。安装了 VictoriaMetrics 时，优先级为 warning 及以上的告警也会发到集群的通知渠道，不会发到项目的渠道。

## 安装前准备

每个节点的内核要有 BTF，也就是 Linux 5.8 以上。确认每个节点都有 `/sys/kernel/btf/vmlinux`。

## 使用方法

在 Zeabur 中，服务的**概览**会列出它的**运行时告警**。没有 Namespace 的告警，出现在**集群** → **概览**的**节点告警**。详见[运行时告警（Falco）](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/diagnosis/#运行时告警falco)。要把告警发到 Slack 或邮箱，详见[将集群告警发送到通知渠道](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/notifications/)。

## 移除

移除会删除 Falco DaemonSet 和 falcosidekick，检测随之停止。告警存在 `PolicyReport` 对象中，先用 `kubectl get policyreport -A -o yaml` 复制需要的内容。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### Falco 的 Pod 起不来

查看 Pod 和 Falco 的日志。没有 BTF 的节点加载不了探针：

```sh
kubectl -n falco get pods
kubectl -n falco logs ds/falco -c falco
```

### Zeabur 没有显示告警

确认 falcosidekick 在运行，日志里没有写入错误：

```sh
kubectl -n falco logs deploy/falco-falcosidekick
```
