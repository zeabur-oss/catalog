## Introduction

[NGINX](https://nginx.org/) is a web server and reverse proxy.

This template runs one NGINX behind your domain. It answers every request with one line of plain text and serves no files. You change what it does by editing its configuration in a ConfigMap.

## Getting started

1. In Zeabur OSS, open the `nginx` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**. It is the domain you entered in the install form, for example `nginx.example.com`.
2. The page shows the text `Hello from Nginx deployed on Zeabur!`. There is no sign-in, and the template creates no Secret.
3. To serve something else, see Configuration.

## Configuration

### Change what NGINX does

The whole `nginx.conf` is in ConfigMap `nginx`. NGINX reads it only at start. Open the project's **Objects** page, open ConfigMap `nginx`, switch to the YAML view, click **Edit**, change it, then review and apply. Then click **Restart** at the top of the `nginx` service page.

The root filesystem is read-only. Keep `listen 8080`, the `pid` line and the `*_temp_path` lines, which point to `/tmp`. See [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html) in the NGINX docs.

### Forward requests to another service

Replace the `return` line in `location /` with a `proxy_pass` line that names a Service in the same namespace:

```nginx
location / {
  proxy_pass http://my-app:3000;
}
```

See "Setting Up a Simple Proxy Server" in the [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html).

## Troubleshooting

### The page shows 502 or does not load

Open the `nginx` service in Zeabur OSS. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica is not ready, open **Deployments** and switch to **Events**. If Events show `OOMKilled`, the service ran out of memory. Raise **Maximum memory** under **Settings** → **Compute**. If the domain or the certificate stays pending, see [Troubleshooting](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting) on the domains page.

See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/) in the Zeabur OSS docs.

### The new Pod does not start after an edit

NGINX refuses to start when the configuration has an error. The old Pod keeps serving until the new one is ready, so the site stays up. Open the **Logs** tab of the `nginx` service, pick the new replica with the selector at the top, and read the line NGINX rejected. Fix the ConfigMap on the **Objects** page, then click **Restart** again.
