## Introduction

External Secrets Operator copies secrets from an external store, such as OpenBao, Vault, 1Password or AWS Secrets Manager, into Kubernetes Secrets. It keeps them in sync at the interval you set. Services read the result like any other Secret.

The component is optional. It does nothing until you add a SecretStore. The webhook certificate comes from cert-manager, so the chart's own certificate controller is off.

The operator reads, creates, updates and deletes Secrets in every namespace. It also creates ServiceAccount tokens in every namespace, so a SecretStore can sign in as a ServiceAccount.

## Before you install

- Install cert-manager first. It issues the webhook certificate.
- Prepare your secret store and read-only credentials for it.

## Use it

Add a SecretStore in the namespace of the service, then an ExternalSecret that names the values to copy. See [Sync secrets from an external store](https://oss.zeabur.com/docs/zeabur/cluster/external-secrets/). It has steps for OpenBao, Vault, 1Password and AWS Secrets Manager.

## Remove it

Removing the chart also deletes the CRDs, and with them every SecretStore and ExternalSecret. With the default `creationPolicy: Owner`, an ExternalSecret owns its Secret, so the Secret is deleted too and services lose the value. Before you remove it, set `creationPolicy: Orphan` on each ExternalSecret whose Secret must stay, or copy the Secrets. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### An ExternalSecret is not ready

Run `kubectl -n <namespace> describe externalsecret <name>`. The status says whether the SecretStore is the problem. Then run `kubectl -n <namespace> describe secretstore <name>` to see if it can reach the store.

### Creating a SecretStore fails

Writes to its CRDs go through the webhook. Check it with `kubectl -n external-secrets get pods` and `kubectl -n external-secrets get certificate`.
