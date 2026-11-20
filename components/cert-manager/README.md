## Introduction

cert-manager requests TLS certificates from an issuer and renews them before they expire. A cluster needs it for HTTPS on Zeabur public domains.

The component creates two Let's Encrypt ClusterIssuers, `letsencrypt` and `letsencrypt-staging`. Both use HTTP-01 through Traefik and set no email address. Try `letsencrypt-staging` first: its rate limits are far higher, but browsers do not trust its certificates. The component also turns on Gateway API support, so cert-manager can issue certificates for Gateway listeners, and it deletes a TLS Secret together with its Certificate.

cert-manager reads and writes Secrets in every namespace and updates the CA bundles of webhooks.

## Before you install

- Install an ingress component first, such as Traefik Proxy. cert-manager needs its Gateway API CRDs.
- For HTTP-01, the domain's DNS record must point to the cluster, and port 80 must be reachable from the internet. The issuers also need outbound access to Let's Encrypt.
- A wildcard domain, such as `*.example.com`, needs a DNS-01 issuer. You create it yourself. See the page below.

## Use it

In Zeabur, add a public domain on the service's **Networking** tab. When the cluster has more than one issuer, the form shows an **Issuer** field. See [Add public domains and HTTPS](https://oss.zeabur.com/docs/zeabur/configure/domains/). The same page shows how to create a DNS-01 issuer for wildcard domains.

## Remove it

Removing cert-manager stops issuing and renewing. Existing Certificates, TLS Secrets and the CRDs stay, but the certificates expire when nobody renews them. The two ClusterIssuers go. The account key Secrets `letsencrypt-account-key` and `letsencrypt-staging-account-key` stay in the `cert-manager` namespace.

Zeabur Builder and External Secrets Operator need it. Zeabur refuses to remove cert-manager while either is installed. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### A certificate does not become ready

Run `kubectl get certificate -A`, then `kubectl -n <namespace> describe certificate <name>`. Then check the challenges with `kubectl get challenges -A`. For HTTP-01, the domain must reach port 80 of the cluster from the internet. See the [cert-manager ACME troubleshooting guide](https://cert-manager.io/docs/troubleshooting/acme/).

### A ClusterIssuer is not ready

Run `kubectl get clusterissuer` and `kubectl describe clusterissuer letsencrypt`. An issuer is ready when its ACME account is registered. That needs outbound access to Let's Encrypt.
