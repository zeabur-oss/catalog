## Introduction

[Halo](https://github.com/halo-dev/halo) is a website and blog system with themes and plugins. This template runs Halo behind your domain, with a PostgreSQL database.

## Getting started

1. In Zeabur, open the `halo` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `halo.example.com`. Visitors see the site there, and the console is at `/console`.
2. The first start creates the super administrator `admin`. There is no setup wizard. Get its password. Open **Variables**, click `halo-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur, read Secret `halo-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret halo-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Open `https://halo.example.com/console` and sign in as `admin` with that password. Then change the password in the console.

## Configuration

### Install a theme

In the console, open Themes, then Theme management at the top right. You can install a theme from the built-in app store, upload a theme package, or download one from a URL. After you install it, select it and click Enable. See [Themes](https://docs.halo.run/guide/use/themes).

### Back up the data

Halo has its own backup. In the console, open Backup and click Create backup. Halo runs the backup in the background. Backup files are in the `backups` folder of the Halo work directory, which is on the volume `halo-data` (`/var/lib/halo`). Store a copy somewhere else too.

Halo keeps its content in the `halo-db` database. In the project, open `halo-db`. Under **Backups**, set up S3 backups. To get a file instead, click **More** → **Export**.

A large backup file can fail to upload in the browser. Copy it into the `backups` folder with kubectl, then restore it from the backup files in the console:

```sh
kubectl -n <namespace> cp ./<backup file> halo-0:/var/lib/halo/backups/
```

See [Backup and restore](https://docs.halo.run/guide/use/backup) in the Halo docs. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) and [Download a database dump](https://oss.zeabur.com/docs/zeabur/deploy/databases/#download-a-database-dump) in the Zeabur docs.

## Troubleshooting

### The page shows 502 or does not load

Open the `halo` service in Zeabur. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

Without Zeabur, read the logs with `kubectl -n <namespace> logs statefulset/halo` and the events with `kubectl -n <namespace> get events --sort-by=.lastTimestamp`.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur docs.
