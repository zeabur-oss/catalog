## Introduction

[NGINX](https://nginx.org/) is a web server and reverse proxy.

This template runs one NGINX behind your domain. It answers every request with one line of plain text and serves no files. You change what it does by editing its configuration in a ConfigMap.

## Getting started

1. Wait until the service shows as running, then open its domain, for example `https://nginx.example.com`.
2. The page shows the text `Hello from Nginx deployed on Zeabur!`. There is no sign-in, and the template creates no Secret.
3. To serve something else, see Configuration.

## Configuration

### Change what NGINX does

The whole `nginx.conf` is in ConfigMap `nginx`. The Pod reads it only at start, so restart the Deployment after you edit it:

```sh
kubectl -n <namespace> edit configmap nginx
kubectl -n <namespace> rollout restart deployment/nginx
```

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

### The new Pod does not start after an edit

NGINX refuses to start when the configuration has an error. The old Pod keeps serving until the new one is ready, so the site stays up. Read the log of the new Pod for the line NGINX rejected, fix the ConfigMap, and restart the Deployment again:

```sh
kubectl -n <namespace> get pods
kubectl -n <namespace> logs <pod name>
```
