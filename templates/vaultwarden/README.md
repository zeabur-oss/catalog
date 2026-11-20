## Introduction

[Vaultwarden](https://github.com/dani-garcia/vaultwarden) is a password manager that works with Bitwarden apps and browser extensions.

This template runs one Vaultwarden server behind your domain. Sign-up is closed. The administrator invites each person on the admin page, which needs a token that is generated at deploy time.

## Getting started

1. Wait until the service shows as running, then note its domain, for example `https://vault.example.com`.
2. Get the admin token. It is in Secret `vaultwarden-secrets`, key `admin-token`:

   ```sh
   kubectl -n <namespace> get secret vaultwarden-secrets -o jsonpath='{.data.admin-token}' | base64 -d
   ```

3. Open `https://<your domain>/admin` and enter the token.
4. Invite each person by email on that page.
5. Each invited person opens `https://<your domain>` and registers with the invited email. Then they sign in from the Bitwarden apps, using your domain as the server address.

## Configuration

### Change the admin token

Write a new value to `vaultwarden-secrets`, key `admin-token`, then restart the StatefulSet so it reads the new value:

```sh
kubectl -n <namespace> rollout restart statefulset/vaultwarden
```

The token is the password of the admin page. See [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page).

## Troubleshooting

### The admin page asks for the token again

An admin session lasts 20 minutes by default. Enter the token again. See [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page).

### Invited people get no email

This template sets no SMTP server, so Vaultwarden sends no email. Tell each person yourself. They can register with the invited email address.
