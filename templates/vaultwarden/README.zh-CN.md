## 简介

[Vaultwarden](https://github.com/dani-garcia/vaultwarden) 是密码管理器，可以搭配 Bitwarden 的应用和浏览器扩展使用。

这个模板在你的域名后面运行一个 Vaultwarden 服务器。注册功能是关闭的，由管理员在管理页面逐一邀请用户。管理页面需要令牌，令牌在部署时自动生成。

## 开始使用

1. 等服务显示为运行中，记下它的域名，例如 `https://vault.example.com`。
2. 获取管理员令牌。令牌在机密 `vaultwarden-secrets` 的 `admin-token` 字段：

   ```sh
   kubectl -n <namespace> get secret vaultwarden-secrets -o jsonpath='{.data.admin-token}' | base64 -d
   ```

3. 打开 `https://<你的域名>/admin`，输入令牌。
4. 在这个页面用邮箱逐一邀请用户。
5. 受邀的人打开 `https://<你的域名>`，用受邀的邮箱注册。之后在 Bitwarden 的应用里，把你的域名当成服务器地址登录。

## 配置

### 更换管理员令牌

把新的值写进 `vaultwarden-secrets` 的 `admin-token` 字段，再重启 StatefulSet，让它读到新值：

```sh
kubectl -n <namespace> rollout restart statefulset/vaultwarden
```

这个令牌就是管理页面的密码。详见 [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page)。

## 故障排除

### 管理页面又要求输入令牌

管理员会话默认只有 20 分钟，请重新输入令牌。详见 [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page)。

### 受邀的人没有收到邮件

这个模板没有配置 SMTP 服务器，所以 Vaultwarden 不会发邮件。请自己通知每位用户，他们可以用受邀的邮箱注册。
