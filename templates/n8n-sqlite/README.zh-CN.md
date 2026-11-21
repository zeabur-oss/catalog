## 简介

[n8n](https://github.com/n8n-io/n8n) 是工作流自动化工具。你用节点把应用和服务串起来，n8n 会按触发条件或计划运行工作流。

这个模板在你的域名后面运行一个 n8n，数据用 SQLite 存在卷上。Code 节点在独立的 `task-runner` Pod 里运行。所有者账号会在启动时建好，所以不会出现设置界面。

## 开始使用

1. 在 Zeabur 开源版打开 `n8n` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，就是你在安装表单填的域名，例如 `n8n.example.com`。
2. 用你在安装表单填的所有者邮箱登录。密码从 `n8n` 服务获取：打开**变量**，在**来自配置文件和机密变量**中点击 `n8n-secrets`，再点击 `owner-password` 旁的眼睛图标。不用 Zeabur 开源版时，用 kubectl 读取机密变量 `n8n-secrets` 的 `owner-password`：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.owner-password}' | base64 -d
   ```

3. 把加密密钥另外备份到安全的地方。n8n 用这把密钥加密它保存的凭据。获取方式相同：在**变量**中点击 `n8n-secrets`，再点击 `encryption-key` 旁的眼睛图标。不用 Zeabur 开源版时，用 kubectl 读取机密变量 `n8n-secrets` 的 `encryption-key`：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.encryption-key}' | base64 -d
   ```

4. 机密 `n8n-secrets` 里还有 `runner-token`，这是 n8n 和 task runner 彼此信任用的共享机密。日常使用不需要它。获取方式同上。不用 Zeabur 开源版时，用 kubectl：

   ```sh
   kubectl -n <namespace> get secret n8n-secrets -o jsonpath='{.data.runner-token}' | base64 -d
   ```

## 配置

### 更换所有者密码

n8n 每次启动都会用安装表单和机密变量设置所有者账号，并在界面里把所有者信息显示为只读。要换密码，在 `n8n` 服务的**变量**中，于**来自配置文件和机密变量**点击 `n8n-secrets`，填入新的 `owner-password`，检查后应用，再点击服务页面顶部的**重启**。

详见 n8n 文档的 [Change the instance owner email](https://docs.n8n.io/deploy/host-n8n/configure-n8n/change-instance-owner-email)。

### 用 Webhook 调用工作流

Webhook 节点有两个 URL。测试 URL 在你选 Listen for Test Event 时有效，并会在编辑器显示收到的数据。生产 URL 在你发布工作流后有效。两个 URL 都以你的域名开头。详见 n8n 文档的 [Webhook node](https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.webhook/)。

### 备份或迁移数据

n8n 把 SQLite 数据库和文件放在 `n8n-data` 卷上，挂载在 `/home/node/.n8n`。n8n 建议先停掉它再复制 SQLite 文件，所以请改用 n8n 的命令行导出工作流和凭证。

1. 留好“开始使用”里的加密密钥。没有它，n8n 读不了已保存的凭证。
2. 导出工作流和凭证。点击 `n8n` 服务页面顶部的**终端**，运行：

   ```sh
   n8n export:workflow --backup --output=/home/node/.n8n/backups/workflows/
   n8n export:credentials --backup --output=/home/node/.n8n/backups/credentials/
   ```

   接着到 `n8n` 服务的**文件**标签页，下载这两个文件夹。不要下载整个卷，文件夹太大会超时。

3. 复制工作流保存的文件。这个模板把它们放在卷的 `/home/node/.n8n/binaryData`，数据库和导出都不包含它们。这个文件夹可能很大，所以用 kubectl 直接传到你的电脑，不要用**文件**标签页：

   ```sh
   kubectl -n <namespace> exec deploy/n8n -c n8n -- busybox tar czf - -C /home/node/.n8n binaryData > n8n-binary-data.tgz
   ```

   要放回新的 `n8n`，运行 `kubectl -n <namespace> exec -i deploy/n8n -c n8n -- busybox tar xzf - -C /home/node/.n8n < n8n-binary-data.tgz`。详见 n8n 文档的 [Binary data environment variables](https://docs.n8n.io/hosting/configuration/environment-variables/binary-data/)。

要迁到另一个 n8n，先在新 `n8n` 服务的**文件**标签页上传这两个文件夹，再用**终端**导入：

```sh
n8n import:workflow --separate --input=<workflows folder>
n8n import:credentials --separate --input=<credentials folder>
```

新的 n8n 必须用同一个加密密钥，否则读不了凭证。做不到时，在 `n8n export:credentials` 加上 `--decrypted`。这个文件里的凭证是明文，导入后要删掉。导出的内容不含用户、执行记录、变量和实例设置。

详见 n8n 文档的 [Back up and restore](https://docs.n8n.io/deploy/host-n8n/keep-n8n-running/backup-and-restore) 和 [Use the command line](https://docs.n8n.io/deploy/host-n8n/configure-n8n/use-the-command-line)。

## 故障排除

### 页面显示 502、打不开或一直重新连接

在 Zeabur 开源版打开 `n8n` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。服务很慢或一直重新连接，也常是内存不够，同样检查**事件**和**内存上限**。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

详见 Zeabur 开源版文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

### Code 节点不运行

Code 节点在 `task-runner` 服务里运行，它用 `runner-token` 连接 n8n。在 Zeabur 开源版打开 `task-runner` 服务，确认**概览**显示副本已就绪，再看**日志**标签页。

详见 n8n 文档的 [Task runners](https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners)。
