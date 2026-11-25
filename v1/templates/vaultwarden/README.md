## Introduction

[Vaultwarden](https://github.com/dani-garcia/vaultwarden) is a password manager that works with Bitwarden apps and browser extensions.

This template runs one Vaultwarden server behind your domain. Sign-up is closed. The administrator invites each person on the admin page, which needs a token that is generated at deploy time.

## Getting started

1. In Zeabur OSS, open the `vaultwarden` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**; it is the domain you entered in the install form.
2. Get the admin token. Open **Variables**, click `vaultwarden-secrets` under **From config files and secrets**, and click the eye icon next to `admin-token`. Without Zeabur OSS, read Secret `vaultwarden-secrets`, key `admin-token`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret vaultwarden-secrets -o jsonpath='{.data.admin-token}' | base64 -d
   ```

3. Open `https://<your domain>/admin` and enter the token.
4. Invite each person by email on that page.
5. Each invited person opens `https://<your domain>` and registers with the invited email. Then they sign in from the Bitwarden apps, using your domain as the server address.

## Configuration

### Change the admin token

On the service's **Variables** tab, click `vaultwarden-secrets`, enter a new value for `admin-token`, then review and apply. The running replica keeps the old value until you click **Restart** at the top of the service page. The token is the password of the admin page. See [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page).

## Troubleshooting

### The page shows 502 or does not load

In Zeabur OSS, open the `vaultwarden` service. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica keeps restarting, open **Deployments**, then **Events**. If the events show `OOMKilled`, raise **Maximum memory** in **Settings** → **Compute**. See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/).

If the domain or its certificate stays pending, see [Add public domains and HTTPS](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting).

### The admin page asks for the token again

An admin session lasts 20 minutes by default. Enter the token again. See [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page).

### Invited people get no email

This template sets no SMTP server, so Vaultwarden sends no email. Tell each person yourself. They can register with the invited email address.
