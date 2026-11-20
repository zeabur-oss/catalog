## 简介

[SillyTavern](https://github.com/SillyTavern/SillyTavern) 是大语言模型的聊天前端。它连接你选的模型 API，并保存你的角色和对话。

这个模板在你的域名后面运行一个 SillyTavern。访问受一个 Basic 认证密码保护，密码在部署时自动生成。对话和设置存在卷里。

## 开始使用

1. 在 Zeabur 打开 `sillytavern` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**，也就是你在安装表单填的域名。
2. 浏览器会要求登录。用户名是 `user`。密码在机密变量 `app-secrets` 的 `basic-password`。打开**变量**，在**来自配置文件和机密变量**点 `app-secrets`，再点 `basic-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `app-secrets` 的 `basic-password`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.basic-password}' | base64 -d
   ```

3. 没有另外的用户账号。登录后，配置你的模型 API 连接（见“配置”）。

## 配置

### 连接模型 API

点顶部栏的 API Connections，选择 API 类型和来源，输入你的 API 密钥，再点 Connect。详见 [Quick Start](https://docs.sillytavern.app/usage/quick-start/) 和 [API Connections](https://docs.sillytavern.app/usage/api-connections/)。

### 更改密码

在服务的**变量**标签页点 `app-secrets`，填入新的 `basic-password`，检查后应用。运行中的副本会继续用旧值，直到你点服务页顶部的**重启**。下次打开页面时，浏览器会要求输入新密码。

## 故障排除

### 浏览器一直要求登录

用户名用 `user`，密码用 `app-secrets` 的 `basic-password` 字段。更改密码之后，浏览器保存的旧密码就不能用了。
