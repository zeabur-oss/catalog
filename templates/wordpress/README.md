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
