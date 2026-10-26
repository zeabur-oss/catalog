# Template standards

Every template in Zeabur Catalog meets these rules. `catalog validate`, `catalog
template lint` and the installation test enforce many of them; check the rest
when you add or change a template. Follow the upstream documentation for the
exact versions the template pins.

## template.yaml

- `metadata` names the application, its license, homepage and categories.
- Each service names its security profile (`restricted` or `baseline`), its
  resource requests and limits, and its ports. A service has the name of its
  Deployment, StatefulSet or database object. Its CPU and memory are the sum of
  that object's containers, and its ports are their named ports; `catalog
  template lint` checks both.
- `security`, `scalability` and `upgradeNotes` tell an operator what to know
  before deploying: risks with their mitigations, the replica range and what
  blocks more replicas, and the steps an upgrade needs.
- `smoke` holds checks that prove the application works: a resource condition,
  and an HTTP or SQL check against the running service.
- `requires` lists the components the template needs, as `<capability>` or
  `<capability>/<upstream>`.

## Licenses and images

- Name the license of the application and its images. Use the exact
  license name for source-available software such as BUSL-1.1 or SSPL, and link
  the license in the template description. Do not add an input that asks the
  person to type an acceptance string.
- Pin every image, including init, migration and backup images, to a digest, and
  keep the readable tag next to it.
- Install nothing at runtime: no package managers, no downloads of mutable code,
  no cluster-wide installers inside an application template.

## Pod security

- Run every container, including init containers and Jobs, under the
  [Restricted Pod Security Standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/):
  non-root user, `allowPrivilegeEscalation: false`, all capabilities dropped and
  `seccompProfile: RuntimeDefault`.
- Use a read-only root filesystem where the application allows it, with
  dedicated writable volumes for the paths it writes.
- A `baseline` service gives a `baselineJustification` that names the control it
  fails and why. Never use privileged containers, host namespaces, `hostPath` or
  host ports.
- `catalog template lint` allows only the sysctls
  `net.ipv4.ip_unprivileged_port_start` and `net.ipv4.ping_group_range`, and
  rejects probes or hooks that set a host.
- Do not change the namespace's Pod Security labels.
- Check pods that an operator creates (database instances, backup Jobs) against
  the same rules, using the operator's documented fields.

## Identity and Secrets

- Set `automountServiceAccountToken: false` and grant no RBAC unless the
  application calls the Kubernetes API. Scope any access to named resources and
  verbs in the namespace.
- Rendered resources reference Secrets; they never contain passwords, keys or
  tokens. Declare generated values as `generate` inputs and existing credentials
  as required inputs.
- Give every input that a person gives a short `label`, and a label and
  description in `locales` for zh-TW and zh-CN. Write them for the person
  deploying, in everyday words.
- Write the `security` summary and every risk in plain words that a person who
  self-hosts for the first time understands: what can go wrong, and for whom.
  Give each of them `locales` for zh-TW and zh-CN.
- Turn off unauthenticated admin and debug endpoints and insecure first-run
  defaults. Say how the first administrator signs in.

## Network

- Give every pod the label `templates.zeabur.com/template: <id>`. For operator
  pods, set it in the custom resource: `inheritedMetadata` for CloudNativePG,
  `podMetadata` for Dragonfly and MariaDB, and `replsets[].labels` for Percona.
- Do not ship default-deny or per-pod NetworkPolicies. The installer isolates
  project namespaces from each other with a baseline policy, and that is the
  default boundary.
- Exception: a component with no sign-in or key of its own gets an ingress
  NetworkPolicy that admits only its own app. Examples: Redis or Dragonfly
  without a password, Meilisearch or Qdrant without a key, and sandboxes that
  run other people's code. List its service in `security.unauthenticated`;
  `catalog template lint` then requires an ingress NetworkPolicy whose
  `podSelector` names that service, and the `security` risks say why it
  exists. A component that runs untrusted code may also limit egress to DNS
  and its own peers.
- Do not limit egress otherwise. Applications call model providers, webhooks,
  mail servers and S3 stores that the template cannot know, and NetworkPolicy
  matches IP ranges, not host names.
- Make every NetworkPolicy, and every peer in the template's namespace, select
  pods by the label above. Other applications can share the namespace, and the
  template's policies must not change their traffic.
- People who want stricter isolation inside a namespace can add their own
  NetworkPolicy. For example, this admits only pods labelled `role=frontend`
  to the pods of one template:

  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata: {name: only-frontend}
  spec:
    podSelector:
      matchLabels: {templates.zeabur.com/template: ghost}
    policyTypes: [Ingress]
    ingress:
    - from: [{podSelector: {matchLabels: {role: frontend}}}]
  ```
- Keep databases, metrics and admin interfaces private. Expose only the
  application through the ingress component with a cert-manager certificate.

## Labels

- Set `metadata.version` to the tag of the application's main image. The
  renderer adds `app.kubernetes.io/instance: <id>`,
  `app.kubernetes.io/version: <version>` and `templates.zeabur.com/template:
  <id>` to every object it renders and to the Secrets of generated inputs, so
  `kubectl delete <kinds> -l app.kubernetes.io/instance=<id>` removes one
  install from its namespace.

## Data and backups

- Give each piece of state its owner: database, uploaded files, search indexes,
  queues, configuration and encryption keys. Use `emptyDir` only for data that
  can be lost.
- Use a database operator from the components where one exists: CloudNativePG for PostgreSQL, Percona for MongoDB, MariaDB Operator for MariaDB and
  Dragonfly Operator for Redis-compatible use.
- Do not turn on database backups in the template. New installs start without
  them, and people turn them on later from the database's Backups tab in Zeabur.
  Say in `upgradeNotes` where to turn them on, and what a restore needs
  (database, files, Secrets and keys).
- If a template does configure backups, send them to HTTPS S3 with TLS
  verification and credentials in a Secret.
- Say what deleting the application keeps and what it destroys.
- Give each S3 endpoint an optional CA input, a Secret with `ca.crt`, for
  storage behind a private CA, when its client takes one. The e2e test uses
  it. Manifests have no conditions: mount the Secret with `optional: true`, and
  give the CA file path its own input with a `default`.
- Write only field substitution in `manifests.yaml`. Express a choice as an
  input with a `default`, not as a condition.

## Scaling and resources

- Set CPU and memory requests and limits on every container, Job and sidecar.
- State the supported replica range and what blocks more replicas: node-local
  volumes, an embedded database, in-memory sessions, a singleton scheduler.
- Use a startup probe for slow starts, readiness to stop traffic, and liveness
  only for a state a restart fixes.
- Add a PodDisruptionBudget to replicated services. Enable autoscaling only for
  services that support it.

## Upgrades

- Write the order of an upgrade into `upgradeNotes`: back up, migrate, roll out.
  Say who runs schema migrations and whether old and new versions can run at the
  same time.
- Say when a rollback needs a restore. Reverting an image does not revert a
  database migration.
- Upgrade a shared operator through its component, never from a template.

## Tests

- The smoke checks run in CI on kind with the components the template
  requires: in the merge queue on a cluster shared with tests that need the
  same components, and every night on a fresh cluster. Make them prove the main behaviour, such as
  an authenticated request or a database query, beyond a Ready pod.
- Use only disposable credentials and storage in tests. Put test input values
  in `config/e2e.yaml`. Object storage goes to SeaweedFS in the test cluster.
