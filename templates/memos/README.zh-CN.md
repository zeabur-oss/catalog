## 简介

[Memos](https://usememos.com/) 是记短笔记的笔记服务，有标签、搜索和 API。你的笔记存在自己的服务器上。

这个模板在你的域名后面运行一个 Memos。数据用 SQLite 存在 10 GiB 的卷上。公开注册已关闭，第一次启动时会建好名为 `admin` 的管理员账号。

## 开始使用

1. 在 Zeabur 打开 `memos` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，就是你在安装表单填的域名，例如 `memos.example.com`。
2. 以 `admin` 登录。密码从 `memos` 服务获取：打开**变量**，在**来自配置文件和机密变量**中点击 `memos-secrets`，再点击 `admin-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `memos-secrets` 的 `admin-password`：

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

配置文件声明的每一组设置，Memos 都会在界面和 API 里设为只读。模板在 ConfigMap `memos-config` 里声明了常规设置（General），其中包含注册规则和自定义的个人资料。Memos 把文件当成整组设置，没有写的字段会回到默认值。要修改这些设置，请打开项目的**对象**页面，打开 ConfigMap `memos-config`，切换到 YAML 视图，点击**编辑**修改，检查后应用。Memos 在启动时读取这个文件，所以要点击 `memos` 服务页面顶部的**重启**。

详见 Memos 文档的 [Declarative configuration](https://usememos.com/docs/configuration/declarative-configuration)。

### 登录失败

Memos 的用户名区分大小写，请输入小写的 `admin`。详见 Memos 文档的 [Troubleshooting](https://usememos.com/docs/troubleshooting)。
