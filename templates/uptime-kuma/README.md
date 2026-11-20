## Introduction

[Uptime Kuma](https://github.com/louislam/uptime-kuma) watches your services and tells you when they go down. It also publishes status pages.

This template runs one Uptime Kuma instance behind your domain. Its data is on a volume. There is no generated password and no Secret.

## Getting started

1. Wait until the service shows as running, then open its domain right after the deploy, for example `https://kuma.example.com`.
2. The first visitor sees the setup page and creates the owner account. Do this before anyone else does, because whoever visits first becomes the owner.
3. Add your first monitor.

## Configuration

### Get notified when a monitor goes down

Add a notification in the settings and attach it to your monitors. Uptime Kuma supports many services, and Apprise adds more. See [Notification Methods](https://uptime.kuma.pet/docs/Notification-Methods).

### Publish a status page

A status page is meant for public users. It caches results for 5 minutes, so it updates more slowly than the dashboard. See [Status Page](https://uptime.kuma.pet/docs/Status-Page).

## Troubleshooting

### Ping monitors fail

Ping monitors use unprivileged ICMP sockets, and the container has no `NET_RAW` capability. Whether ping works depends on how your cluster network handles ICMP. Use an HTTP, TCP or DNS monitor where ping fails.

### You forgot the password

Run the reset command in the pod and follow the prompts:

```sh
kubectl -n <namespace> exec -it uptime-kuma-0 -- npm run reset-password
```

See [Reset Password via CLI](https://uptime.kuma.pet/docs/Reset-Password-via-CLI).
