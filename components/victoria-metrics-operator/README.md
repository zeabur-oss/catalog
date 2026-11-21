## Introduction

The VictoriaMetrics operator creates and manages the metrics and log stores from custom resources such as `VMSingle` and `VLSingle`. It also reads Prometheus operator objects, so existing app manifests keep working.

You do not install it on its own. VictoriaMetrics and VictoriaLogs need it, and Zeabur OSS installs it first. It needs cert-manager (or another component that provides `certificates`), which issues the certificate for its admission webhook.

It manages its objects in every namespace. It can read every Secret, create Pods in any namespace and create ClusterRoles and bindings, so treat its token as cluster-admin.

## Use it

The operator has no Zeabur OSS screen. It starts to work when you install VictoriaMetrics or VictoriaLogs. Check that it runs and list what it manages:

```sh
kubectl -n observability get deploy victoria-metrics-operator
kubectl get vmsingle,vlsingle -A
```

To write your own objects, such as scrape targets and alert rules, see the [operator documentation](https://docs.victoriametrics.com/operator/).

## Remove it

Before it goes, a cleanup Job deletes every VictoriaMetrics and VictoriaLogs object in every namespace. This includes stores, agents, alert rules, scrape objects, probes and the notification channels stored as `VMAlertmanagerConfig`. Remove VictoriaMetrics and VictoriaLogs first, and keep a copy of any rules or channels you want to reuse.

The CRDs stay, and so does the cleanup hook's ClusterRole and ClusterRoleBinding. Zeabur OSS refuses to remove it while another installed component needs it. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### Creating a VictoriaMetrics object fails with a webhook error

The admission webhook needs its certificate. Check that it is ready and that the operator Pod runs:

```sh
kubectl -n observability get certificate victoria-metrics-operator-validation
kubectl -n observability get pods
```
