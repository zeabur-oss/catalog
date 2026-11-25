## Introduction

Prometheus Blackbox Exporter checks from inside the cluster whether a site or a port answers. It ships two modules. `http_2xx` expects a 2xx answer after redirects and tries IPv6 first, then IPv4. `tcp_connect` opens a TCP connection. Both give up after 5 seconds.

Install it when you want uptime checks and alerts for your own targets. It needs VictoriaMetrics, which stores the results. On several nodes it runs two replicas, so one failing replica does not mean the target is down. It has no Kubernetes API access and makes only outbound probes.

## Use it

Blackbox Exporter has no Zeabur OSS screen. Tell the metrics agent what to probe with a `VMProbe`. This example checks a site every minute:

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

The metric `probe_success` is `1` while the target answers. To alert on it, write a `VMRule`. See the [VMProbe](https://docs.victoriametrics.com/operator/resources/vmprobe/) and [VMRule](https://docs.victoriametrics.com/operator/resources/vmrule/) pages.

## Remove it

Removal deletes the exporter. Your `VMProbe` objects stay, and their probes fail until you delete them. Results already stored stay until VictoriaMetrics' retention removes them. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A probe always fails

Run the same probe by hand with debug output:

```sh
kubectl -n observability port-forward svc/blackbox-exporter-prometheus-blackbox-exporter 9115:9115
curl 'localhost:9115/probe?module=http_2xx&target=https://example.com&debug=true'
```

The output shows each step, such as DNS, the connection and the redirects.
