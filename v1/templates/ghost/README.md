## Introduction

[Ghost](https://github.com/TryGhost/Ghost) is a publishing platform for blogs and paid newsletters. This template runs Ghost 6 behind your domain, with a MySQL database that MOCO runs.

Ghost sends sign-in and system email through the SMTP server you entered in the install form.

## Getting started

1. In Zeabur OSS, open the `ghost` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `blog.example.com`. The admin is at `/ghost` on that domain.
2. A setup job creates the site owner from the owner email you entered in the install form, so there is no setup form. Get the owner password. On the `ghost` service, open **Variables**, click `ghost-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur OSS, read Secret `ghost-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret ghost-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Open the admin page and sign in with the owner email and that password.

The Secret `ghost-mysql-app`, key `password`, holds the password of the `ghost` MySQL user that Ghost connects as. MOCO keeps its own administrator passwords in `moco-ghost-mysql`.

## Configuration

### Send newsletters

SMTP only sends system email. To send a post as a newsletter, Ghost needs a bulk email provider, and Mailgun is the only one it supports. In the admin, open the email newsletter settings and enter your Mailgun API keys. See [Newsletters](https://docs.ghost.org/newsletters) and [Why do I have to set up Mailgun?](https://docs.ghost.org/faq/mailgun-newsletters).

### Back up the data

Ghost keeps posts, members and settings in the `ghost-mysql` database. In the project, open `ghost-mysql`. Under **Backups**, set up S3 backups. To get a file instead, click **More** → **Export**. Images, themes and other uploads are on the volume `ghost-data`, mounted at `/var/lib/ghost/content`.

Copy the content folder with the **Files** tab of the `ghost` service. For a large folder, use kubectl:

```sh
kubectl -n <namespace> cp ghost-0:/var/lib/ghost/content ./ghost-content
```

The export in the Ghost admin, under Settings > Advanced > Import/Export, gives posts and settings as JSON, but not the images of a self-hosted site. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) in the Zeabur OSS docs and [Exporting content and data](https://ghost.org/help/exports/) in the Ghost docs.

### Change the database password

Ghost reads the password from `ghost-mysql-app`, but MySQL keeps its own copy. If you change only the Secret, Ghost cannot connect. Change it in MySQL first, as `moco-writable`:

```sh
kubectl -n <namespace> exec -i moco-ghost-mysql-0 -c mysqld -- mysql -h127.0.0.1 -umoco-writable \
  -p"$(kubectl -n <namespace> get secret moco-ghost-mysql -o jsonpath='{.data.WRITABLE_PASSWORD}' | base64 -d)" \
  -e "ALTER USER 'ghost'@'%' IDENTIFIED BY '<new password>'"
```

Then enter the same value for `password` in `ghost-mysql-app`, review and apply, and click **Restart** on the `ghost` service.

## Troubleshooting

### Ghost restarts after the install

Ghost cannot connect until MOCO has started MySQL and the job `ghost-mysql-app-setup` has made the `ghost` user. This takes a few minutes, and Ghost restarts until then. Check the database with `kubectl -n <namespace> get mysqlcluster ghost-mysql` and the job with `kubectl -n <namespace> get job ghost-mysql-app-setup`.

### Sign-in or system email fails with a connection timeout

Some hosts block outbound SMTP ports. Use the SMTP port that your provider and your network allow. See [Configuration](https://docs.ghost.org/config).

### The page shows 502 or does not load

Open the `ghost` service in Zeabur OSS. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the service is slow or keeps reconnecting, open the **Usage** tab. **Near memory max** means the memory is close to the limit. Raise **Maximum memory** the same way. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

Without Zeabur OSS, read the logs with `kubectl -n <namespace> logs statefulset/ghost` and the events with `kubectl -n <namespace> get events --sort-by=.lastTimestamp`.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur OSS docs.
