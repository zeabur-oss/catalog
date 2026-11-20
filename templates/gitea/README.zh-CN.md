## 简介

[Gitea](https://about.gitea.com/) 是 Git 托管服务，提供仓库、议题和合并请求。这个模板在你的域名后面运行 Gitea，仓库放在卷上，搭配 PostgreSQL 数据库。

公开注册已关闭，SSH 也关闭了。大家用 HTTPS 使用 Gitea。

## 开始使用

1. 在 Zeabur 打开 `gitea` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `git.example.com`。不会出现安装向导。
2. 第一次启动时会创建管理员 `gitea_admin`。获取它的密码。打开**变量**，在**来自配置文件和机密变量**中点击 `gitea-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `gitea-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret gitea-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 用 `gitea_admin` 和这个密码登录，并在 Gitea 里修改密码。Gitea 只在第一次启动时读取这个机密，之后再改机密不会生效。

`gitea-secrets` 里其他的字段（`secret-key`、`internal-token`、`jwt-secret` 和 `lfs-jwt-secret`）是 Gitea 自己使用。`secret-key` 要在整个数据库的使用期间保留，因为 Gitea 用它加密双因素验证密钥等已保存的凭证。

## 配置

### 创建用户

注册已关闭，其他用户由管理员创建。打开 Site Administration，再打开 User Accounts。详见 Gitea 的 [FAQ](https://docs.gitea.com/help/faq)。

### 用访问令牌通过 HTTPS 推送

在 Settings > Applications 创建访问令牌，也可以直接打开 `https://git.example.com/user/settings/applications`。给它需要的权限，并在 Gitea 显示时复制下来，因为 Gitea 只显示一次。Git 要求密码时，把令牌当作密码输入：

```sh
git clone https://git.example.com/<用户>/<仓库>.git
```

详见 [API Usage](https://docs.gitea.com/development/api-usage)。

## 故障排除

### 用 SSH 克隆仓库失败

这个模板关闭了 SSH。请使用仓库的 HTTPS 克隆地址。详见 [Config Cheat Sheet](https://docs.gitea.com/administration/config-cheat-sheet)。
