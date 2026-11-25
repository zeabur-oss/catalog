## Introduction

[Gitea](https://about.gitea.com/) is a Git hosting service with repositories, issues and pull requests. This template runs Gitea behind your domain, with repositories on a volume and a PostgreSQL database.

Public registration is off, and SSH is off. People use Gitea over HTTPS.

## Getting started

1. In Zeabur OSS, open the `gitea` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `git.example.com`. There is no install wizard.
2. The first start creates the administrator `gitea_admin`. Get its password. Open **Variables**, click `gitea-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur OSS, read Secret `gitea-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret gitea-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Sign in as `gitea_admin` with that password. Change the password in Gitea. Gitea reads the Secret only at the first start, so a later change of the Secret does not apply.

The other keys in `gitea-secrets` (`secret-key`, `internal-token`, `jwt-secret` and `lfs-jwt-secret`) are for Gitea's own use. Keep `secret-key` for the life of the database, because Gitea uses it to encrypt stored credentials such as two-factor secrets.

## Configuration

### Create a user

Registration is off, so the administrator creates the other users. Open Site Administration, then User Accounts. See the [FAQ](https://docs.gitea.com/help/faq) of Gitea.

### Push over HTTPS with an access token

Create an access token in Settings > Applications, or at `https://git.example.com/user/settings/applications`. Give it the permissions you need, and copy it when Gitea shows it, because Gitea shows it only once. Use the token as the password when Git asks:

```sh
git clone https://git.example.com/<user>/<repository>.git
```

See [API Usage](https://docs.gitea.com/development/api-usage).

### Back up the data

Gitea keeps accounts, issues and settings in the `gitea-db` database. In the project, open `gitea-db`. Under **Backups**, set up S3 backups. To get a file instead, click **More** → **Export**. Repositories and uploaded files are on the volume `gitea-data`, mounted at `/var/lib/gitea`. Copy it with the **Files** tab of the `gitea` service. For a large folder, use kubectl. `kubectl -n <namespace> get pods` lists the replica names:

```sh
kubectl -n <namespace> cp <replica>:/var/lib/gitea ./gitea-data
```

Gitea also has a `gitea dump` command that packs the repositories, the database and the data into one ZIP file. Gitea says to stop the instance during the backup, so that the files and the database match. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) and [Download a database dump](https://oss.zeabur.com/docs/zeabur/deploy/databases/#download-a-database-dump) in the Zeabur OSS docs and [Backup and Restore](https://docs.gitea.com/administration/backup-and-restore) in the Gitea docs.

## Troubleshooting

### Cloning over SSH fails

SSH is off in this template. Use the HTTPS clone address of the repository. See [Config Cheat Sheet](https://docs.gitea.com/administration/config-cheat-sheet).

### The page shows 502 or does not load

Open the `gitea` service in Zeabur OSS. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

Without Zeabur OSS, read the logs with `kubectl -n <namespace> logs deployment/gitea` and the events with `kubectl -n <namespace> get events --sort-by=.lastTimestamp`.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur OSS docs.
