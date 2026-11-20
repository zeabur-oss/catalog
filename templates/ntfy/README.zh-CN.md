## 简介

[ntfy](https://ntfy.sh/) 让脚本和应用通过普通的 HTTP 请求，把推送通知发到你的手机或桌面。你把消息发到某个主题，订阅这个主题的所有客户端都会收到。

这个模板在你的域名后面运行一个 ntfy 服务器。每个主题都需要账号，消息和用户存在 5 GiB 的卷上。每次启动都会建好名为 `admin` 的用户。

## 开始使用

1. 等服务显示为运行中，打开它的域名，例如 `https://ntfy.example.com`。
2. 以 `admin` 登录。密码在机密 `ntfy-secrets` 的 `admin-password` 字段：

   ```sh
   kubectl -n <namespace> get secret ntfy-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 所有主题默认都拒绝访问，所以要发送或订阅的应用，必须用这个账号或访问令牌登录。请见“配置”。

## 配置

### 更换 admin 密码

ntfy 每次启动都会应用这个机密。要换密码，把新的值写进 `ntfy-secrets` 的 `admin-password` 字段，再重启 StatefulSet：

```sh
kubectl -n <namespace> patch secret ntfy-secrets --type merge -p '{"stringData":{"admin-password":"<新密码>"}}'
kubectl -n <namespace> rollout restart statefulset/ntfy
```

### 添加用户

在 Pod 里运行 `ntfy user add`，再用 `ntfy access` 让用户能访问某个主题。`rw` 表示可以发送也可以订阅：

```sh
kubectl -n <namespace> exec -it statefulset/ntfy -- ntfy user add <用户名>
kubectl -n <namespace> exec statefulset/ntfy -- ntfy access <用户名> <主题> rw
```

详见 ntfy 文档的 [Access control](https://docs.ntfy.sh/config/)。

### 创建访问令牌

访问令牌让应用不用你的密码就能发送或订阅。登录网页版后，在 Account 区域就能创建。也可以在 Pod 里创建：

```sh
kubectl -n <namespace> exec statefulset/ntfy -- ntfy token add --label=<标签> admin
```

把令牌当作 Bearer 令牌发送：

```sh
curl -H "Authorization: Bearer tk_..." -d "Hello" https://ntfy.example.com/mytopic
```

详见 ntfy 文档的 [Publishing](https://docs.ntfy.sh/publish/)。

## 故障排除

### 客户端无法发送或订阅

模板默认拒绝访问每个主题。请让客户端用用户名和密码，或访问令牌登录。`admin` 以外的用户，还要用 `ntfy access` 让它能访问该主题。详见 ntfy 文档的 [Access control](https://docs.ntfy.sh/config/)。
