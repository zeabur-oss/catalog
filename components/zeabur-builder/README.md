## Introduction

Zeabur Builder builds container images from Git repositories and uploaded folders, with a Dockerfile or Railpack. It keeps the images in zot, an OCI registry that the component installs with it, so builds need no external registry account. Zeabur OSS needs it to deploy services from Git. Without it, services deploy from images only.

Builder runs one buildkitd per namespace that builds. The component adds a CA through cert-manager for zot's certificate. Pushes need a short-lived token that Builder signs. A NetworkPolicy lets only Builder, buildkitd and build pods reach zot. zot keeps the 20 most recent tags of each repository, and it stores images on a 20Gi volume.

Builder reads, creates, updates and deletes Secrets in every namespace (Git credentials and their copies), and manages Builds, BuildRuns and Jobs there. Each buildkitd is privileged and runs with a user namespace and an egress policy.

## Before you install

- Install cert-manager first. It issues zot's certificates.
- Nodes pull images from zot at the fixed address of the `clusterIp` input, a free address in the cluster's Service CIDR. Zeabur picks one when you leave it empty. On K3s, RKE2 and KubeSolo, the `zot-node` DaemonSet points each node at that address and gives it the CA from the `zot-tls` Secret. On other distributions, set up a registry mirror for `zot.zot.svc:5000` to that address on each node, and trust that CA.

## Use it

In a Zeabur OSS project, select **Add** → **Deploy from Git** and enter the repository. See [Build and deploy a service from Git](https://oss.zeabur.com/docs/zeabur/deploy/git/). The page also covers uploading a folder. Keel deploys the finished image, so install it too.

### Turn on the AI builder

The AI builder (`builderRef: agent`) has a model write a Dockerfile for a repository that has none and that Railpack can't build. The model reads the repository's files, so they go to the model provider, and each Dockerfile it writes uses tokens. Later builds reuse that Dockerfile and don't call the model. The builder is off by default. See [Build with the AI builder](https://github.com/zeabur-oss/builder/blob/v0.3.1/docs/builds/ai-builder.mdx).

1. Choose how runs sign in to the model API, and create what that needs in every namespace with a Build on the AI builder: a Secret with the API key, or for a token exchange, a ServiceAccount named in `agentAuth`.
2. Set the component's inputs: `agentEnabled` to `true`, `agentProvider` (the provider's name in Rig, such as `anthropic` or `openai`), `agentModel` and `agentAuth`. Set `agentBaseUrl` only to send requests to a proxy or a server you run. The component then installs a ClusterBuilder named `agent`.
3. Set a Build's `spec.builderRef` to `agent`. The Zeabur OSS interface can't pick the AI builder yet ([zeabur-oss/zeabur#1021](https://github.com/zeabur-oss/zeabur/issues/1021)).

`agentAuth` is the ClusterBuilder's `spec.agent.model.auth`. An API key in the key `anthropic` of the Secret `model`:

```yaml
type: apiKey
apiKey: {name: model, key: anthropic}
```

Anthropic and OpenAI can exchange a service account token instead, so no namespace keeps an API key: `type: anthropicFederation` or `type: openaiFederation`. Any other OAuth 2.0 token endpoint works with `type: tokenExchange`; for Azure OpenAI with Microsoft Entra ID:

```yaml
type: tokenExchange
tokenExchange:
  serviceAccountName: builder-agent
  audience: api://AzureADTokenExchange
  url: https://login.microsoftonline.com/<tenant>/oauth2/v2.0/token
  subjectField: client_assertion
  form:
    grant_type: client_credentials
    client_id: <application client ID>
    client_assertion_type: urn:ietf:params:oauth:client-assertion-type:jwt-bearer
    scope: https://cognitiveservices.azure.com/.default
```

The ClusterBuilder is not a Secret, so put no secret in `form`. The page linked above lists the fields of each type and how to register the cluster with the provider.

## Remove it

Removing Zeabur Builder stops all builds. The `zot` volume stays with every image in it, and so do the CRDs with every Build.

Check first that no service pulls its image from zot. A pod that restarts or moves to another node cannot pull it after zot is gone. See [Remove components](https://oss.zeabur.com/docs/zeabur/cluster/components/#remove-components).

## Troubleshooting

### zot does not start

zot waits for the Secrets `zot-tls` and `zot-token-signer`. Run `kubectl -n zot get certificate,pods`. A Certificate that is not ready points to cert-manager.

### A service cannot pull its image

Pods stay in `ImagePullBackOff`. Run `kubectl describe pod <name>` and read the event. On K3s, RKE2 and KubeSolo, check that a `zot-node` pod runs on that node and that `/var/lib/rancher/k3s/agent/etc/containerd/certs.d/zot.zot.svc:5000/hosts.toml` (RKE2: `rancher/rke2`, KubeSolo: `/var/lib/kubesolo/containerd/registry/`) names the `clusterIp` address.

### A run on the AI builder waits with `ModelKeyMissing`

The Secret, its key or the ServiceAccount that `agentAuth` names is missing from the Build's namespace. Run `kubectl -n <namespace> get buildrun` and read the message, then create what it names. The run starts by itself.
