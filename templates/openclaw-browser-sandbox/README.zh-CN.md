## 简介

这个模板运行一个无头 Chrome，供 [OpenClaw](https://docs.openclaw.ai/tools/browser/remote) 或其他服务通过 Chrome DevTools 协议（CDP）操作。

这个浏览器只给单个可信任的所有者使用。它没有网页，不用登录，也没有令牌。只有同一个项目里带有信任调用方标签的 Pod 连得进来。

## 开始使用

1. 在 Zeabur 打开 `openclaw-sandbox-browser` 服务。**概览**显示副本已就绪，服务就启动好了。它没有域名。
2. 确认你的客户端 Pod 带有标签 `app.kubernetes.io/name`，值要和你在安装表单填的信任调用方相同。搭配 OpenClaw 时，这个值是 `openclaw`。
3. 在该 Pod 里，把下面的地址当成 CDP 端点：

   ```text
   http://openclaw-sandbox-browser:9222
   ```

   没有这个标签的 Pod 会被挡掉。这里没有自动生成的机密变量，所以能连进来的对象都能完全控制浏览器。

## 配置

### 连接 OpenClaw

把 `browser.profiles.<name>.cdpUrl` 设成上面的地址，并让该配置文件成为默认：

```json5
{
  browser: {
    enabled: true,
    defaultProfile: "sandbox",
    profiles: {
      sandbox: { cdpUrl: "http://openclaw-sandbox-browser:9222" },
    },
  },
}
```

地址用 `http://` 开头时，OpenClaw 会通过 `/json/version` 找到 WebSocket 端点。详见 [Remote and hosted browsers](https://docs.openclaw.ai/tools/browser/remote)。

## 故障排除

### 客户端连不上

网络策略只放行带有信任调用方标签的 Pod，而且只开 9222 端口。确认客户端在同一个项目里，且它的 `app.kubernetes.io/name` 标签和你在安装表单填的值相同。详见 [Remote and hosted browsers](https://docs.openclaw.ai/tools/browser/remote)。
