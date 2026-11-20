## 简介

[Vaultwarden](https://github.com/dani-garcia/vaultwarden) 是密码管理器，可以搭配 Bitwarden 的应用和浏览器扩展使用。

这个模板在你的域名后面运行一个 Vaultwarden 服务器。注册功能是关闭的，由管理员在管理页面逐一邀请用户。管理页面需要令牌，令牌在部署时自动生成。

## 开始使用

1. 在 Zeabur 打开 `vaultwarden` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**，也就是你在安装表单填的域名。
2. 获取管理员令牌。打开**变量**，在**来自配置文件和机密变量**点 `vaultwarden-secrets`，再点 `admin-token` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `vaultwarden-secrets` 的 `admin-token`：

   ```sh
   kubectl -n <namespace> get secret vaultwarden-secrets -o jsonpath='{.data.admin-token}' | base64 -d
   ```

3. 打开 `https://<你的域名>/admin`，输入令牌。
4. 在这个页面用邮箱逐一邀请用户。
5. 受邀的人打开 `https://<你的域名>`，用受邀的邮箱注册。之后在 Bitwarden 的应用里，把你的域名当成服务器地址登录。

## 配置

### 更换管理员令牌

在服务的**变量**标签页点 `vaultwarden-secrets`，填入新的 `admin-token`，检查后应用。运行中的副本会继续用旧值，直到你点服务页顶部的**重启**。这个令牌就是管理页面的密码。详见 [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `vaultwarden` 服务，确认**概览**显示副本已就绪，再看**日志**选项卡。副本一直重启时，打开**部署记录**，切换到**事件**。如果事件显示 `OOMKilled`，到**设置** → **计算资源**调高**内存上限**。详见[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

域名或证书一直处于等待中时，看[添加公开域名与 HTTPS](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)的“遇到问题”。

### 管理页面又要求输入令牌

管理员会话默认只有 20 分钟，请重新输入令牌。详见 [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page)。

### 受邀的人没有收到邮件

这个模板没有配置 SMTP 服务器，所以 Vaultwarden 不会发邮件。请自己通知每位用户，他们可以用受邀的邮箱注册。
