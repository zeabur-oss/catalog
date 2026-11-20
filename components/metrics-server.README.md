## Introduction

Kubernetes Metrics Server serves the `metrics.k8s.io` API with the CPU and memory each Pod and node uses right now. It collects the numbers from the kubelets every 15 seconds. It keeps no history, so it is the cheapest source of live usage.

Install it for the live usage in Zeabur, for `kubectl top` and for autoscaling. The Horizontal Pod Autoscaler stops while the API is down. On several nodes it runs two replicas with a PodDisruptionBudget, so one stays up while a node drains.

It reads Pods, nodes and namespaces and the kubelet's node metrics, and it registers an aggregated API. It adds a NetworkPolicy that lets the API server reach it on port 10250. It runs under the restricted Pod Security level.

## Use it

In Zeabur, a service's **Usage** tab shows **Current usage** from this API. Without it, the tab offers **Install**. See [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/#view-usage).

Check it with kubectl:

```sh
kubectl top pods -A
```

## Remove it

Removal deletes the metrics API. `kubectl top`, the live usage in Zeabur and autoscaling then have no data. Zeabur refuses to remove it while another installed component, such as the Vertical Pod Autoscaler, needs it. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### kubectl top says the metrics API is not available

Check that the API is available and that the Pods are ready:

```sh
kubectl get apiservice v1beta1.metrics.k8s.io
kubectl -n kube-system get pods -l app.kubernetes.io/name=metrics-server
```
