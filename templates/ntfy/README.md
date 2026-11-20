## Introduction

[ntfy](https://ntfy.sh/) sends push notifications to your phone or desktop from scripts and apps, through plain HTTP requests. You publish a message to a topic, and every client that subscribes to the topic gets it.

This template runs one ntfy server behind your domain. Every topic needs an account, and messages and users are kept on a 5 GiB volume. A user named `admin` is created at every start.

## Getting started

1. In Zeabur, open the `ntfy` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `ntfy.example.com`.
2. Sign in as `admin`. Get the password from the `ntfy` service: open **Variables**, click `ntfy-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur, read Secret `ntfy-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret ntfy-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Every topic denies access by default, so an app that publishes or subscribes must sign in with this account or with an access token. See Configuration.

## Configuration

### Change the admin password

ntfy applies the Secret at every start. To change the password, open **Variables** on the `ntfy` service, click `ntfy-secrets` under **From config files and secrets**, and enter a new value for `admin-password`. Review and apply, then click **Restart** at the top of the service page.

### Add a user

Click **Terminal** at the top of the `ntfy` service page. Run `ntfy user add`, then give the user access to a topic with `ntfy access`. `rw` means the user can publish and subscribe:

```sh
ntfy user add <username>
ntfy access <username> <topic> rw
```

See [Access control](https://docs.ntfy.sh/config/) in the ntfy docs.

### Create an access token

An access token lets an app publish or subscribe without your password. Create one in the web app, in the Account section after you sign in. You can also click **Terminal** at the top of the `ntfy` service page and run:

```sh
ntfy token add --label=<label> admin
```

Send the token as a Bearer token:

```sh
curl -H "Authorization: Bearer tk_..." -d "Hello" https://ntfy.example.com/mytopic
```

See [Publishing](https://docs.ntfy.sh/publish/) in the ntfy docs.

## Troubleshooting

### The page shows 502 or does not load

Open the `ntfy` service in Zeabur. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur docs.

### A client cannot publish or subscribe

The template denies access to every topic by default. Make the client sign in with a user name and password or an access token. For a user other than `admin`, also run `ntfy access` to give it access to the topic. See [Access control](https://docs.ntfy.sh/config/) in the ntfy docs.
