## Introduction

Stakater Reloader restarts a Deployment, StatefulSet or DaemonSet when a ConfigMap or Secret it uses changes. It restarts only workloads that carry the annotation `reloader.stakater.com/auto: "true"` or `reloader.stakater.com/reload`, so installing it restarts nothing by itself.

Install it when services should pick up new shared variables or Secrets without a manual restart. It reads every ConfigMap and Secret in the cluster and can patch Deployments, StatefulSets and DaemonSets in every namespace. It leaves Jobs and CronJobs alone.

## Use it

In Zeabur OSS, open the service's **Settings** → **Compute**, turn on **Restart when its config files or secrets change**, then review and apply. Zeabur OSS sets the annotation for you. See [Variables](https://oss.zeabur.com/docs/zeabur/configure/variables/).

For a workload outside Zeabur OSS, add the annotation yourself:

```sh
kubectl -n <namespace> annotate deployment <name> reloader.stakater.com/auto=true
```

## Remove it

Removing Reloader stops the automatic restarts. The annotations stay on the workloads and do nothing until Reloader is back. After you remove it, restart a service yourself when its ConfigMap or Secret changes. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A service still uses the old value

Check that the service has the option on, or that the workload carries the annotation (`kubectl -n <namespace> get deployment <name> -o jsonpath='{.metadata.annotations}'`). Then check that Reloader runs: `kubectl -n reloader get pods`, and read its logs with `kubectl -n reloader logs deploy/reloader-reloader`.
