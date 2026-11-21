## Introduction

[Uptime Kuma](https://github.com/louislam/uptime-kuma) watches your services and tells you when they go down. It also publishes status pages.

This template runs one Uptime Kuma instance behind your domain. Its data is on a volume. There is no generated password and no Secret.

## Getting started

1. In Zeabur OSS, open the `uptime-kuma` service. When **Overview** shows its replica as ready, the service is up. Its domain is on the **Networking** tab under **Public domains**; it is the domain you entered in the install form.
2. Open the domain right after the deploy. The first visitor sees the setup page and creates the owner account. Do this before anyone else does, because whoever visits first becomes the owner.
3. Add your first monitor.

## Configuration

### Get notified when a monitor goes down

Add a notification in the settings and attach it to your monitors. Uptime Kuma supports many services, and Apprise adds more. See [Notification Methods](https://uptime.kuma.pet/docs/Notification-Methods).

### Publish a status page

A status page is meant for public users. It caches results for 5 minutes, so it updates more slowly than the dashboard. See [Status Page](https://uptime.kuma.pet/docs/Status-Page).

## Troubleshooting

### The page shows 502 or does not load

In Zeabur OSS, open the `uptime-kuma` service. Check that **Overview** shows its replica as ready, then read the **Logs** tab. If the replica keeps restarting, open **Deployments**, then **Events**. If the events show `OOMKilled`, raise **Maximum memory** in **Settings** → **Compute**. See [Adjust replicas, resources, and health checks](https://oss.zeabur.com/docs/zeabur/configure/service-settings/) and [View logs, events, and usage, and open a terminal](https://oss.zeabur.com/docs/zeabur/configure/logs-terminal/).

If the domain or its certificate stays pending, see [Add public domains and HTTPS](https://oss.zeabur.com/docs/zeabur/configure/domains/#troubleshooting).

### Ping monitors fail

Ping monitors use unprivileged ICMP sockets, and the container has no `NET_RAW` capability. Whether ping works depends on how your cluster network handles ICMP. Use an HTTP, TCP or DNS monitor where ping fails.

### You forgot the password

Click **Terminal** at the top of the service page and run the reset command, then follow the prompts:

```sh
npm run reset-password
```

Without Zeabur OSS, run it with kubectl:

```sh
kubectl -n <namespace> exec -it uptime-kuma-0 -- npm run reset-password
```

See [Reset Password via CLI](https://uptime.kuma.pet/docs/Reset-Password-via-CLI).
