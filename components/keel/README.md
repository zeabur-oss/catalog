## Introduction

Keel updates a workload to a new image tag. It acts only on Deployments and other workloads that carry the annotation `keel.sh/policy`, so installing it changes nothing by itself.

Zeabur needs it for **Auto-deploy new builds**. Zeabur Builder calls the Keel webhook (`http://keel.keel.svc:9300/v1/webhooks/native`) after each push. For workloads annotated `keel.sh/trigger: poll`, Keel also polls the registry every 5 minutes. A workload can set `keel.sh/pollSchedule` to poll faster.

The component turns the Helm provider off, so Keel never rewrites Helm release values. The webhook takes no credentials, so its Service is ClusterIP and a NetworkPolicy lets only the `builder-system` and `keel` namespaces reach port 9300.

Keel reads every Secret (for registry credentials), Namespace and Node. It reads, updates and deletes Pods, ReplicaSets, Deployments, StatefulSets, DaemonSets, Jobs and CronJobs in every namespace.

## Use it

In Zeabur, a service built from Git gets the annotation when you leave **Auto-deploy new builds** on in its **Builds** tab. See [New commits and automatic deployment](https://oss.zeabur.com/docs/zeabur/deploy/git/#new-commits-and-automatic-deployment).

For a workload outside Zeabur, add the annotation yourself:

```sh
kubectl -n <namespace> annotate deployment <name> keel.sh/policy=force
```

## Remove it

Removing Keel stops automatic rollouts. The annotations stay on the workloads and do nothing until Keel is back. Zeabur Builder keeps building, but services no longer switch to new images by themselves. The NetworkPolicy `keel-webhook` goes with it. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A new image does not roll out

Check that the workload has the annotation: `kubectl -n <namespace> get deployment <name> -o jsonpath='{.metadata.annotations}'`. Then check that Keel runs with `kubectl -n keel get pods`, and read its logs with `kubectl -n keel logs deploy/keel`.

### A webhook call times out

The NetworkPolicy `keel-webhook` allows only pods in the `builder-system` and `keel` namespaces. Calls from other namespaces are dropped.
