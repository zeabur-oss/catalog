## 简介

[n8n](https://github.com/n8n-io/n8n) 是工作流自动化工具。你用节点把应用和服务串起来，n8n 会按触发条件或计划运行工作流。

这个模板在你的域名后面运行一个 n8n，并附带专属的 PostgreSQL 数据库。Code 节点在独立的 `n8n-runners` Pod 里运行，这个 Pod 也能运行 Python。所有者账号会在启动时建好，所以不会出现设置界面。

## 开始使用

1. 等服务显示为运行中，打开它的域名，例如 `https://n8n.example.com`。
2. 用你在安装表单填的所有者邮箱登录。密码在机密 `app-secrets` 的 `owner-password` 字段：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. 把加密密钥另外备份到安全的地方。n8n 用这把密钥加密它保存的凭据。密钥在机密 `app-secrets` 的 `encryption-key` 字段：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. 机密 `app-secrets` 里还有两个日常使用不需要的字段。`runner-token` 是 n8n 和代码执行器彼此信任用的共享机密，`jwt-key` 是用户管理用的机密：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.runner-token}' | base64 -d
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.jwt-key}' | base64 -d
   ```

## 配置

### 更换所有者密码

n8n 每次启动都会用安装表单和机密设置所有者账号，并在界面里把所有者信息显示为只读。要换密码，把新的值写进 `app-secrets` 的 `owner-password` 字段，再重启 Deployment：

```sh
kubectl -n <namespace> patch secret app-secrets --type merge -p '{"stringData":{"owner-password":"<新密码>"}}'
kubectl -n <namespace> rollout restart deployment/n8n
```

详见 n8n 文档的 [Change the instance owner email](https://docs.n8n.io/deploy/host-n8n/configure-n8n/change-instance-owner-email)。

### 用 Webhook 调用工作流

Webhook 节点有两个 URL。测试 URL 在你选 Listen for Test Event 时有效，并会在编辑器显示收到的数据。生产 URL 在你发布工作流后有效。两个 URL 都以你的域名开头。详见 n8n 文档的 [Webhook node](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook/)。

## 故障排除

### Code 节点不运行

Code 节点在 `n8n-runners` Pod 里运行，这个 Pod 用 `runner-token` 连回 n8n。请确认 Pod 在运行中，并查看它的日志：

```sh
kubectl -n <namespace> get pods
kubectl -n <namespace> logs deployment/n8n-runners
```

详见 n8n 文档的 [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners)。

### Code 节点里的代码连不上网络

`n8n-runners` Pod 只连得上 DNS 和 n8n 的任务代理，因为模板用 NetworkPolicy 限制了它的网络。在 Code 节点里调用外部地址的代码会失败。请改用 HTTP Request 节点获取数据，再交给 Code 节点。
