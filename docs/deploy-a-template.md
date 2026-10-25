# Deploy a template

Build the `catalog` command from a source checkout you trust, at the release
tag you deploy. Do not run a binary downloaded from a release before you verify
it.

## 1. Verify the release

Download `bundle.tar.gz` and `bundle.tar.gz.sigstore.json` of one release tag
into a new private directory, then check the signature with the exact workflow
identity of that tag:

```sh
tag=vX.Y.Z
cosign verify-blob bundle.tar.gz --bundle bundle.tar.gz.sigstore.json \
  --certificate-identity "https://github.com/zeabur-oss/catalog/.github/workflows/release.yml@refs/tags/$tag" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

An issuer alone, or a loose regular expression, accepts signatures from other
workflows. Unpack the bundle only after the signature passes, and reject
absolute paths, `..` and links.

## 2. Prepare the cluster

- Install each component in the template's `requires` from the same release.
- Use a CNI that enforces NetworkPolicy. Templates deny all traffic in the
  namespace, then allow only what each service needs.
- Mark one StorageClass as the default. Templates name no StorageClass, so
  their volumes use the default. Without one, their claims stay `Pending`.
- For a template with domains, have a cert-manager ClusterIssuer ready.
- Use a dedicated namespace for each deployment.

## 3. Render and apply

Read the license, inputs, `requires`, security risks, scaling limits and upgrade
notes in `templates/<id>/template.yaml` first. Then render the objects:

```sh
go run ./tools/cmd/catalog template render -namespace app \
  -domain app.example.com -cluster-issuer letsencrypt -values inputs.json <id> > plan.yaml
```

`inputs.json` is a JSON object of string values for the inputs the template
asks for. Rendering checks every required input, including license
acknowledgements. Object storage buckets and credentials must already exist.

Generate the template's passwords and keys as Secrets, then apply:

```sh
umask 077
go run ./tools/cmd/catalog template inputs -namespace app <id> > secrets.json
kubectl create namespace app
kubectl -n app apply -f secrets.json
kubectl -n app apply -f plan.yaml
```

Keep `secrets.json` out of version control. Every run of `inputs` creates new
values: run it once per deployment and reuse the Secrets on upgrades. After you
apply, run the template's smoke checks and your own tests.

### Generated inputs

A generated input has a `secret` (`name` and `key`) and a `validation.pattern`
that matches the whole generated value.

| Field | Meaning |
| --- | --- |
| `generate` | `random`. |
| `generateBytes` | Random bytes before encoding, 12 to 64. Default 32. |
| `generateEncoding` | `base64url` (default, unpadded) or lowercase `hex`. |

An input the user supplies has `required` and no generation fields. 12 bytes in
`base64url` give 16 characters; 16 or 32 bytes in `hex` give 32 or 64
characters.

CloudNativePG creates its own application credentials Secret; templates use it
as is.

## 4. Upgrade and roll back

Record the release, namespace, image references, inputs and Secret names of
each deployment. Before an upgrade, diff the rendered objects, read every
upgrade note in between, take a backup and rehearse the upgrade on a copy.

To roll an application back, deploy the objects of a previous verified release
after you check database and storage compatibility; a database downgrade often
needs a restore. Keep backups outside the application namespace.

## References

- [Cosign blob verification](https://docs.sigstore.dev/cosign/verifying/verify/)
- [GitHub OIDC](https://docs.github.com/en/actions/reference/security/oidc)
- [Kubernetes Secret good practices](https://kubernetes.io/docs/concepts/security/secrets-good-practices/)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
