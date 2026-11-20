## 简介

[OpenList](https://github.com/OpenListTeam/OpenList) 把多个存储服务的文件列在同一个网页里，也能通过 WebDAV 提供。这个模板使用一体式镜像，内含 FFmpeg 和 Aria2。

这个模板在你的域名后面运行一个 OpenList。管理员密码和 Aria2 密码都在部署时自动生成。

## 开始使用

1. 等服务显示为运行中，打开它的域名，例如 `https://openlist.example.com`。
2. 用 `admin` 登录。密码在机密 `openlist-secrets` 的 `admin-password` 字段：

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

   OpenList 每次重启都会重置一次这个密码，所以在网页里改的密码会失效。要改密码，请改机密再重启（见“配置”）。
3. 要用离线下载时，Aria2 令牌在同一个机密的 `aria2-password` 字段：

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.aria2-password}' | base64 -d
   ```

   在 Aria2 设置里，端点填 `http://127.0.0.1:6800/jsonrpc`，令牌填这个值。

## 配置

### 更改管理员密码

把新的值写进 `openlist-secrets` 的 `admin-password` 字段，再重启 StatefulSet，让它读到新值：

```sh
kubectl -n <namespace> rollout restart statefulset/openlist
```

### 使用离线下载

打开管理页面，进入设置，再进入其他，设好临时文件夹。接着填入“开始使用”里取得的 Aria2 端点和令牌。详见 [Offline download](https://doc.oplist.org/guide/advanced/offline-download)。

### 用密码保护文件夹

为路径新增一条 meta 记录，并设置密码。密码要覆盖子文件夹时，勾选密码后面的 Apply to sub folder。通过 WebDAV 访问时，这个密码不会生效。详见 [Meta](https://doc.oplist.org/guide/advanced/meta)。
