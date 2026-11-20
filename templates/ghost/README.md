## Introduction

[Ghost](https://github.com/TryGhost/Ghost) is a publishing platform for blogs and paid newsletters. This template runs Ghost 6 behind your domain, with MySQL on a volume.

Ghost sends sign-in and system email through the SMTP server you entered in the install form.

## Getting started

1. In Zeabur, open the `ghost` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**, for example `blog.example.com`. The admin is at `/ghost` on that domain.
2. A setup job creates the site owner from the owner email you entered in the install form, so there is no setup form. Get the owner password. On the `ghost` service, open **Variables**, click `ghost-secrets` under **From config files and secrets**, and click the eye icon next to `admin-password`. Without Zeabur, read Secret `ghost-secrets`, key `admin-password`, with kubectl:

   ```sh
   kubectl -n <namespace> get secret ghost-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Open the admin page and sign in with the owner email and that password.

The other keys in `ghost-secrets` (`database-password` and `root-password`) are for MySQL. Ghost does not use `root-password`.

## Configuration

### Send newsletters

SMTP only sends system email. To send a post as a newsletter, Ghost needs a bulk email provider, and Mailgun is the only one it supports. In the admin, open the email newsletter settings and enter your Mailgun API keys. See [Newsletters](https://docs.ghost.org/newsletters) and [Why do I have to set up Mailgun?](https://docs.ghost.org/faq/mailgun-newsletters).

## Troubleshooting

### Sign-in or system email fails with a connection timeout

Some hosts block outbound SMTP ports. Use the SMTP port that your provider and your network allow. See [Configuration](https://docs.ghost.org/config).
