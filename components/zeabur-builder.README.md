## Introduction

Zeabur Builder builds container images from Git repositories and uploaded folders, with a Dockerfile or Railpack. It keeps the images in zot, an OCI registry that the component installs with it, so builds need no external registry account. Zeabur needs it to deploy services from Git. Without it, services deploy from images only.

Builder runs one buildkitd per namespace that builds. The component adds a CA through cert-manager for zot's certificate. Pushes need a short-lived token that Builder signs. A NetworkPolicy lets only Builder, buildkitd and build pods reach zot. zot keeps the 20 most recent tags of each repository, and it stores images on a 20Gi volume.

Builder reads, creates, updates and deletes Secrets in every namespace (Git credentials and their copies), and manages Builds, BuildRuns and Jobs there. Each buildkitd is privileged and runs with a user namespace and an egress policy.

## Before you install

- Install cert-manager first. It issues zot's certificates.
- Nodes pull images from zot at the fixed address of the `clusterIp` input, a free address in the cluster's Service CIDR, such as `10.43.0.50` on K3s and RKE2. Nodes need a registry mirror to it (`registries.yaml` on K3s and RKE2) and must trust the CA in the `zot-tls` Secret of the `zot` namespace. Without `clusterIp`, only pods reach the registry.

## Use it

In a Zeabur project, select **Add** → **Deploy from Git** and enter the repository. See [Build and deploy a service from Git](https://oss.zeabur.com/docs/zeabur/deploy/git/). The page also covers uploading a folder. Keel deploys the finished image, so install it too.

## Remove it

Removing Zeabur Builder stops all builds. The `zot` volume stays with every image in it, and so do the CRDs with every Build.

Check first that no service pulls its image from zot. A pod that restarts or moves to another node cannot pull it after zot is gone. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### zot does not start

zot waits for the Secrets `zot-tls` and `zot-token-signer`. Run `kubectl -n zot get certificate,pods`. A Certificate that is not ready points to cert-manager.

### A service cannot pull its image

Pods stay in `ImagePullBackOff`. Run `kubectl describe pod <name>` and read the event. Check that the node has the registry mirror for the `clusterIp` address and trusts the CA from `zot-tls`.
