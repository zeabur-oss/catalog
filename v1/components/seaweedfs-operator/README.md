## Introduction

SeaweedFS Operator runs SeaweedFS object storage from a `Seaweed` object and serves its S3 API. Buckets, users, keys and policies are objects too. Install it when projects need S3 object storage inside the cluster.

This component is in beta. Most of the chart comes from one maintainer, and its features and CRDs still change between releases. The operator needs cert-manager for its webhook certificate.

The operator manages Pods, StatefulSets, Services, Secrets, PVCs, Jobs and Ingresses for `Seaweed` objects in every namespace. It also calls each storage service's admin API. The Grafana dashboard is off.

## Use it

In Zeabur OSS, open a project's **Add** menu and choose **Object storage**. Set the capacity and the first bucket, then connect services with the S3 endpoint and keys. See [Create S3 object storage and buckets](https://oss.zeabur.com/docs/zeabur/deploy/object-storage/).

## Remove it

Removing the operator deletes only the operator and its webhook. The CRDs stay, and so do your `Seaweed` and bucket objects, the Pods the operator created and their PVCs. Nothing manages them until the operator is back. Zeabur OSS refuses to remove the component while `Seaweed` objects exist. Delete object storage in Zeabur OSS first, and back up files you still need. PVCs of deleted storage stay for you to delete by hand. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### The storage service never becomes ready

The operator creates the S3 gateway after the master, volume and filer. Check the progress with `kubectl -n <namespace> get seaweed` and `kubectl -n <namespace> get pods`. Read the operator's logs with `kubectl -n seaweedfs-operator logs deploy/seaweedfs-operator`.

### A `Seaweed` object is rejected by the webhook

The webhook needs its certificate. Check it with `kubectl -n seaweedfs-operator get certificate seaweedfs-operator-webhook-cert`, and check that cert-manager runs.
