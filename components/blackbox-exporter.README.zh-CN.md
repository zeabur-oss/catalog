## 简介

Prometheus Blackbox Exporter 从集群内部检查网站或端口有没有响应。带两个模块。`http_2xx` 会跟随重定向，要求最后得到 2xx 响应，先试 IPv6，再试 IPv4。`tcp_connect` 建立 TCP 连接。两者都在 5 秒后放弃。

想对自己的目标做可用性检查和告警时，就安装它。需要 VictoriaMetrics 来保存结果。多节点时运行两个副本，所以只有一个副本探测失败时，不一定是目标宕机。没有 Kubernetes API 权限，只对外发出探测。

## 使用方法

Blackbox Exporter 没有 Zeabur 界面。用 `VMProbe` 告诉指标代理要探测什么。这个示例每分钟检查一个网站：

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMProbe
metadata:
  name: example-site
  namespace: observability
spec:
  jobName: example-site
  interval: 60s
  module: http_2xx
  vmProberSpec:
    url: blackbox-exporter-prometheus-blackbox-exporter.observability.svc:9115
  targets:
    static:
      targets:
        - https://example.com
```

目标有响应时，指标 `probe_success` 是 `1`。要据此发告警，请编写 `VMRule`。详见 [VMProbe](https://docs.victoriametrics.com/operator/resources/vmprobe/) 和 [VMRule](https://docs.victoriametrics.com/operator/resources/vmrule/)。

## 移除

移除会删除导出器。你创建的 `VMProbe` 会保留，删除之前，探测都会失败。已保存的结果在 VictoriaMetrics 的保留期限到期前都会保留。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 探测一直失败

手动运行同一个探测，并开启调试输出：

```sh
kubectl -n observability port-forward svc/blackbox-exporter-prometheus-blackbox-exporter 9115:9115
curl 'localhost:9115/probe?module=http_2xx&target=https://example.com&debug=true'
```

输出会列出每个步骤，例如 DNS、连接和重定向。
