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

### Back up the site

In the console, open Backup and click Create backup. Halo runs the backup in the background. Backup files are in the `backups` folder of the Halo work directory, which is on the volume `halo-data`. Store a copy somewhere else too. See [Backup and restore](https://docs.halo.run/guide/use/backup).
