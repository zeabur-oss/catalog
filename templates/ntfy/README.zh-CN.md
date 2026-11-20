## 简介

[ntfy](https://ntfy.sh/) 让脚本和应用通过普通的 HTTP 请求，把推送通知发到你的手机或桌面。你把消息发到某个主题，订阅这个主题的所有客户端都会收到。

这个模板在你的域名后面运行一个 ntfy 服务器。每个主题都需要账号，消息和用户存在 5 GiB 的卷上。每次启动都会建好名为 `admin` 的用户。

## 开始使用

1. 在 Zeabur 打开 `ntfy` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，就是你在安装表单填的域名，例如 `ntfy.example.com`。
2. 以 `admin` 登录。密码从 `ntfy` 服务获取：打开**变量**，在**来自配置文件和机密变量**中点击 `ntfy-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `ntfy-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret ntfy-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 所有主题默认都拒绝访问，所以要发送或订阅的应用，必须用这个账号或访问令牌登录。请见“配置”。

## 配置

### 更换 admin 密码

ntfy 每次启动都会应用这个机密变量。要换密码，在 `ntfy` 服务的**变量**中，于**来自配置文件和机密变量**点击 `ntfy-secrets`，填入新的 `admin-password`，检查后应用，再点击服务页面顶部的**重启**。

### 添加用户

点击 `ntfy` 服务页面顶部的**终端**。运行 `ntfy user add`，再用 `ntfy access` 让用户能访问某个主题。`rw` 表示可以发送也可以订阅：

```sh
ntfy user add <用户名>
ntfy access <用户名> <主题> rw
```

详见 ntfy 文档的 [Access control](https://docs.ntfy.sh/config/)。

### 创建访问令牌

访问令牌让应用不用你的密码就能发送或订阅。登录后在网页应用的 Account 区域创建。也可以点击 `ntfy` 服务页面顶部的**终端**，运行：

```sh
ntfy token add --label=<标签> admin
```

把令牌当作 Bearer 令牌发送：

```sh
curl -H "Authorization: Bearer tk_..." -d "Hello" https://ntfy.example.com/mytopic
```

详见 ntfy 文档的 [Publishing](https://docs.ntfy.sh/publish/)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `ntfy` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

详见 Zeabur 文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。

### 客户端无法发送或订阅

模板默认拒绝访问每个主题。请让客户端用用户名和密码，或访问令牌登录。`admin` 以外的用户，还要用 `ntfy access` 让它能访问该主题。详见 ntfy 文档的 [Access control](https://docs.ntfy.sh/config/)。
