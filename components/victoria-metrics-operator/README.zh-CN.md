## 简介

VictoriaMetrics operator 根据 `VMSingle`、`VLSingle` 这类自定义资源，创建并管理指标和日志的存储服务。它也能读取 Prometheus operator 的对象，所以现有的应用清单照样能用。

不用单独安装。VictoriaMetrics 和 VictoriaLogs 需要这个 operator，Zeabur 会先装好。operator 需要 cert-manager（或其他提供 `certificates` 的组件），来签发 admission webhook 的证书。

operator 管理每个 Namespace 的对象，能读取所有 Secret、在任何 Namespace 创建 Pod，也能创建 ClusterRole 和绑定，所以令牌等于 cluster-admin。

## 使用方法

operator 没有 Zeabur 界面。安装 VictoriaMetrics 或 VictoriaLogs 之后 operator 才开始工作。确认 operator 在运行，并列出它管理的对象：

```sh
kubectl -n observability get deploy victoria-metrics-operator
kubectl get vmsingle,vlsingle -A
```

要自己编写抓取目标、告警规则这类对象，详见 [operator 文档](https://docs.victoriametrics.com/operator/)。

## 移除

移除前，清理 Job 会删除所有 Namespace 中的 VictoriaMetrics 和 VictoriaLogs 对象，包括存储服务、代理、告警规则、抓取对象、探测，以及存成 `VMAlertmanagerConfig` 的通知渠道。请先移除 VictoriaMetrics 和 VictoriaLogs，想沿用的规则或渠道先自己备份。

CRD 会保留，清理 hook 的 ClusterRole 和 ClusterRoleBinding 也会保留。其他已安装的组件还需要它时，Zeabur 会拒绝移除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 创建 VictoriaMetrics 对象时出现 webhook 错误

admission webhook 需要证书。确认证书已就绪，operator 的 Pod 也在运行：

```sh
kubectl -n observability get certificate victoria-metrics-operator-validation
kubectl -n observability get pods
```
