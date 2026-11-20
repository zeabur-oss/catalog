## Introduction

[WordPress](https://wordpress.org/) is a website and blog engine. This template runs WordPress with its own MariaDB database, behind your domain. Your files and uploads are kept on a volume, and the database is on its own 10Gi volume.

The site is already installed. The owner account `admin` is created for you, with the administrator email you entered in the install form. Uploads up to 256 MB are allowed.

## Getting started

1. In Zeabur, open the `wordpress` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**; it is the domain you entered in the install form.
2. Sign in at `https://<your domain>/wp-admin` as `admin`. Open **Variables**, click `wordpress-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur, read Secret `wordpress-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. The database is MariaDB. In your project it is the `mariadb` database. WordPress reaches it at Service `mariadb`, port `3306`, with database `zeabur` and user `wordpress`. Connections use TLS. The passwords are in the same Secret, keys `database-password` and `database-root-password`. Open them the same way, or with kubectl:

   ```sh
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.database-password}' | base64 -d
   kubectl -n <namespace> get secret wordpress-secrets -o jsonpath='{.data.database-root-password}' | base64 -d
   ```

4. The same Secret also holds the keys and salts that WordPress uses to sign its login cookies: `auth-key`, `secure-auth-key`, `logged-in-key`, `nonce-key`, `auth-salt`, `secure-auth-salt`, `logged-in-salt` and `nonce-salt`. You do not need them to use the site. Read one with the same command, for example `{.data.auth-key}`.

## Configuration

### Change the admin password

After you sign in, go to Users, All Users, and click your user name. Under New Password, click Generate Password or type your own, then click Update User. The new password works at once. The value in `wordpress-secrets` is used only for the first install, so it does not change. See [Reset your password](https://wordpress.org/documentation/article/reset-your-password/).

### Choose a theme and add plugins

Go to Appearance, Themes to change the look of your site, and to Plugins to add features. See [First steps with WordPress](https://wordpress.org/documentation/article/first-steps-with-wordpress/).

### Set the address format of your pages

Go to Settings, Permalinks, pick a structure and click Save Changes. See [Settings Permalinks screen](https://wordpress.org/documentation/article/settings-permalinks-screen/).

### Back up the data

In Zeabur, open the `mariadb` database and go to **Backups** → **Set up backups** to back up on a schedule to an S3 bucket. To get a copy now, click **More** at the top right of the database page, then **Export**. The browser downloads gzip-compressed SQL made by `mariadb-dump --single-transaction`. See [Set up S3 backups](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) and [Download a database dump](https://oss.zeabur.com/docs/zeabur/deploy/databases/#download-a-database-dump).

The uploads, themes and plugins are not in the database. They are in the `wordpress-data` volume, mounted at `/var/www/html` in the `wordpress` service. Back up `wp-content` together with the database dump. In Zeabur, the **Files** tab can download a folder, but a big folder can time out. For a big `wp-content`, stream it out with kubectl:

```sh
kubectl -n <namespace> exec deploy/wordpress -c wordpress -- tar czf - -C /var/www/html wp-content > wp-content.tar.gz
```

## Troubleshooting

### The page shows 502 or does not load

In Zeabur, open the `wordpress` service. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica keeps restarting, open **Deployments**, then **Events**. If the events show `OOMKilled`, raise **Maximum memory** in **Settings** → **Compute**. See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/).

If the domain or its certificate stays pending, see [Add public domains and HTTPS](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting).
