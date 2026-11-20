## 简介

[Memos](https://usememos.com/) 是记短笔记的笔记服务，有标签、搜索和 API。你的笔记存在自己的服务器上。

这个模板在你的域名后面运行一个 Memos。数据用 SQLite 存在 10 GiB 的卷上。公开注册已关闭，第一次启动时会建好名为 `admin` 的管理员账号。

## 开始使用

1. 等服务显示为运行中，打开它的域名，例如 `https://memos.example.com`。
2. 以 `admin` 登录。密码在机密 `memos-secrets` 的 `admin-password` 字段：

   ```sh
   kubectl -n <namespace> get secret memos-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 登录后在 Memos 里改密码。Memos 只在创建账号时读取这个机密，之后改机密不会改变密码。
4. 注册功能已关闭，其他账号由 `admin` 在 Memos 的设置里创建。

## 配置

### 创建个人访问令牌

在 Memos 的用户设置里创建令牌，再当作 Bearer 令牌发送。令牌以 `memos_pat_` 开头，Memos 只会显示一次。每个应用各用一个令牌，要撤销时才不会影响其他应用。

```sh
curl -H "Authorization: Bearer memos_pat_..." https://memos.example.com/api/memos
```

详见 Memos 文档的 [API Access](https://usememos.com/docs/integrations/api-access)。

## 故障排除

### 设置是只读的，或无法保存

配置文件声明的每一组设置，Memos 都会在界面和 API 里设为只读。模板在 ConfigMap `memos-config` 里声明了常规设置（General），其中包含注册规则和自定义的个人资料。Memos 把文件当成整组设置，没有写的字段会回到默认值。要修改这些设置，请编辑 ConfigMap，再重启 StatefulSet：

```sh
kubectl -n <namespace> edit configmap memos-config
kubectl -n <namespace> rollout restart statefulset/memos
```

详见 Memos 文档的 [Declarative configuration](https://usememos.com/docs/configuration/declarative-configuration)。

### 登录失败

Memos 的用户名区分大小写，请输入小写的 `admin`。详见 Memos 文档的 [Troubleshooting](https://usememos.com/docs/troubleshooting)。
