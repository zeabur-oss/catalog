## Introduction

[Miniflux](https://miniflux.app/) is a minimal feed reader. It fetches your RSS and Atom feeds and shows the new articles in one place.

This template runs one Miniflux behind your domain, with its own PostgreSQL database. An administrator account named `admin` is created at the first start.

## Getting started

1. In Zeabur, open the `miniflux` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `miniflux.example.com`.
2. Sign in as `admin`. Get the password from the `miniflux` service: open **Variables**, click `miniflux-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur, read Secret `miniflux-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret miniflux-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Add your first feed.
4. Change the password in Miniflux if you like. Miniflux reads the Secret only when it creates the account, so a later change to the Secret does not change the password.

## Configuration

### Create an API key

Go to Settings, then API Keys, then Create a new API key. Send the key in the `X-Auth-Token` header:

```sh
curl -H "X-Auth-Token: <key>" https://miniflux.example.com/v1/me
```

See [API](https://miniflux.app/docs/api.html) in the Miniflux docs.

### Use a reader app

Many reader apps can connect to Miniflux through the Google Reader API or the Fever API. In Miniflux, open Settings, then Integrations, and choose a user name and password for the API. Use that user name and password in the app. See [Google Reader API](https://miniflux.app/docs/google_reader.html) and [Fever API](https://miniflux.app/docs/fever.html) in the Miniflux docs for the apps that work.

### Back up the data

Miniflux keeps feeds, articles and settings in the `miniflux-db` database. It keeps nothing on a volume. In the project, open the `miniflux-db` database. Under **Backups**, set up S3 backups. To get a file instead, click **More** → **Export**. See [Create, back up, and restore databases](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) and [Download a database dump](https://oss.zeabur.com/docs/zeabur/deploy/databases/#download-a-database-dump) in the Zeabur docs.

## Troubleshooting

### The page shows 502 or does not load

Open the `miniflux` service in Zeabur. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur docs.

### A feed on a private address is refused

Miniflux refuses to fetch from private and loopback network addresses by default, and the template does not change this. Use a feed address that is reachable from the public internet. See `FETCHER_ALLOW_PRIVATE_NETWORKS` in [Configuration parameters](https://miniflux.app/docs/configuration.html).

### A feed does not refresh when you ask

Miniflux allows a manual refresh only once every 30 minutes by default (`FORCE_REFRESH_INTERVAL`). It also refreshes feeds in the background on its own schedule. See [Configuration parameters](https://miniflux.app/docs/configuration.html).
