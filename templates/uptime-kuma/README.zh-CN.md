## 简介

[Uptime Kuma](https://github.com/louislam/uptime-kuma) 监视你的服务，挂掉时通知你，也能发布状态页面。

这个模板在你的域名后面运行一个 Uptime Kuma，数据存在卷里。这里没有自动生成的密码，也没有机密。

## 开始使用

1. 在 Zeabur 打开 `uptime-kuma` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**，也就是你在安装表单填的域名。
2. 部署完成后马上打开这个域名。第一个进来的人会看到设置页面并创建所有者账号。请抢在别人之前完成，因为谁先进来谁就是所有者。
3. 添加你的第一个监控项。

## 配置

### 监控项挂掉时收到通知

在设置里添加通知，再应用到你的监控项。Uptime Kuma 支持许多通知服务，Apprise 还能再多接一批。详见 [Notification Methods](https://uptime.kuma.pet/docs/Notification-Methods)。

### 发布状态页面

状态页面是给普通用户看的。它会把结果缓存 5 分钟，所以更新比仪表盘慢。详见 [Status Page](https://uptime.kuma.pet/docs/Status-Page)。

## 故障排除

### Ping 监控失败

Ping 监控用的是不需特权的 ICMP socket，容器没有 `NET_RAW` 权限。Ping 能不能用，要看集群网络怎么处理 ICMP。Ping 不通的地方，改用 HTTP、TCP 或 DNS 监控。

### 忘记密码

点服务页顶部的**终端**，运行重置命令，再按提示操作：

```sh
npm run reset-password
```

不用 Zeabur 时，用 kubectl 运行：

```sh
kubectl -n <namespace> exec -it uptime-kuma-0 -- npm run reset-password
```

详见[Reset Password via CLI](https://uptime.kuma.pet/docs/Reset-Password-via-CLI)。
