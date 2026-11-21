## 简介

CrowdSec 读取 Traefik 的访问日志，找出暴力破解和扫描行为，在请求到达服务之前封禁对方的地址。封禁由 Traefik 插件在 `web` 和 `websecure` 入口完成，所以 Ingress 后面的每个应用都受保护，不用各自配置。

集群通过 Traefik 对外提供服务时，就安装它。需要 Traefik 和提供 `certificates` 的组件。安装时也会改动 Traefik：加上插件（Traefik 启动时从 plugins.traefik.io 下载），并在访问日志中保留每个客户端的地址和 User-Agent。

这个组件失效时会放行。CrowdSec API 连不上时，Traefik 继续处理流量。在线 API 是关闭的，所以没有数据离开集群。需要的密钥会自动生成。

每个节点上的代理以特权模式运行，并只读挂载主机的 `/var/log`。代理的客户端证书能发送和读取告警，也能删除封禁决定。

## 使用方法

CrowdSec 没有 Zeabur 开源版界面。列出封禁了什么、看到了什么：

```sh
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions list
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli alerts list
```

要添加或删除封禁决定，详见 [cscli decisions 参考](https://doc.crowdsec.net/docs/cscli/cscli_decisions)。

## 移除

移除会删除代理、本地 API 和 bouncer middleware。存放封禁决定的卷会留在 `crowdsec` Namespace。移除后，用 `kubectl -n traefik get pods` 确认 Traefik 的 Pod 正常。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 正常的访客被封禁

在封禁决定中找到地址，再删除：

```sh
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions list
kubectl -n crowdsec exec deploy/crowdsec-lapi -- cscli decisions delete --ip <地址>
```

### 一直没有封禁任何地址

确认代理在运行，并且读到了 Traefik 的日志：

```sh
kubectl -n crowdsec get pods
kubectl -n crowdsec exec ds/crowdsec-agent -- cscli metrics
```
