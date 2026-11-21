## 简介

VictoriaMetrics 保存集群的指标历史，并评估告警规则。每 30 秒抓取一次指标，单节点保留 7 天，多节点保留 14 天，数据放在 5 Gi 的卷中。Alertmanager 负责发送 Zeabur 开源版的通知：每个项目设置自己的渠道，集群管理员设置集群的渠道。

想在 Zeabur 开源版查看用量历史、接收通知时就安装它。组件会抓取每个 Namespace 的 Pod，也会读取每个 Namespace 的构建、数据库、备份和证书来产生告警，节点导出器运行在主机网络上，读取主机的 `/proc` 和 `/sys`。

这个组件关闭了 Zeabur 开源版用不到的部分：Grafana、默认仪表盘和 API server 指标。能把告警送进 Alertmanager 的只有告警规则、指标代理和 Falco。Falco 的告警只发到集群的渠道。

## 使用方法

在 Zeabur 开源版中，服务的**用量**标签页会显示 VictoriaMetrics 保存的用量历史。详见[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/#查看用量)。到**集群** → **通知**添加渠道并发送测试，详见[将集群告警发送到通知渠道](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/notifications/)。

## 移除

移除会删除指标存储、代理、告警规则、Alertmanager 和导出器。历史数据的卷会留在 `observability` Namespace，不再需要数据时自己删除。

移除后，用量历史和通知都会停止。其他已安装的组件（例如 Blackbox Exporter）还需要它时，Zeabur 开源版会拒绝移除。VictoriaMetrics operator 会保留。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 历史或通知没有数据

确认存储和代理的状态是 operational。两条命令都应该输出 `operational`：

```sh
kubectl -n observability get vmsingle -o jsonpath='{.items[0].status.updateStatus}'
kubectl -n observability get vmagent -o jsonpath='{.items[0].status.updateStatus}'
```

不是的话，用 `kubectl -n observability get pods` 查看 Pod，并确认 operator 在运行。
