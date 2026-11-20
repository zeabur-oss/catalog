## 简介

[Dify](https://github.com/langgenius/dify) 是构建 LLM 应用的平台。你在网页编辑器里设计工作流、智能体和对话应用，再发布成网页应用或 API。

这个模板在你的域名后面运行 Dify，包含 API、后台任务进程、插件服务、代码沙箱、PostgreSQL 和队列。Dify 把文档和上传的文件存在你在安装表单填写的 S3 兼容存储。

## 开始使用

1. 在 Zeabur 打开 `dify` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `dify.example.com`。Dify 的其他服务（例如 `dify-api`）也要就绪。
2. 获取初始化密码。打开 `dify-api` 服务的**变量**，在**来自配置文件和机密变量**中点击 `dify-secrets`，再点击 `init-password` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `dify-secrets` 的 `init-password`：

   ```sh
   kubectl -n <namespace> get secret dify-secrets -o jsonpath='{.data.init-password}' | base64 -d
   ```

3. 打开 `https://dify.example.com/install`。Dify 会先要求输入初始化密码，别人就无法抢先占用这个实例。输入密码后，创建管理员账号。
4. 用刚创建的账号登录 `https://dify.example.com`。

`dify-secrets` 里其他的字段（`app-key`、`redis-password` 和 `sandbox-key`）是 Dify 自己的服务使用，请不要外泄。

## 配置

### 添加模型服务

连上模型服务之前，Dify 没有任何模型可用。在 Dify 打开 Integrations > Model Provider，安装你要用的服务，点 Add API Key，输入你的密钥。Dify 会先检查密钥，通过后才让这个服务可用。详见 [Model Providers](https://docs.dify.ai/en/use-dify/workspace/model-providers)。

### 备份数据

Dify 把账号、应用和设置存在 `dify-db` 数据库，里面有 `dify` 和 `dify_plugin` 两个数据库。在项目中打开 `dify-db`，到**备份**配置 S3 备份，备份 `dify-db` 时两个数据库都包含在内。文档和上传的文件放在你在安装表单填的 S3 存储桶，请用存储服务自己的功能备份它。安装的插件放在卷 `dify-plugin-storage`，挂载在 `/app/storage`。

备份时一并保管机密变量 `dify-secrets`。Dify 用 `app-key` 签署令牌和文件 URL，并加密 OAuth 客户端凭据。详见 Zeabur 文档的[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和 Dify 文档的 [SECRET_KEY](https://docs.dify.ai/en/self-host/configuration/environments)。

## 故障排除

### 页面显示 502 或打不开

在 Zeabur 打开 `dify` 服务。确认**概览**显示副本已就绪，再看**日志**标签页。副本没有就绪时，打开**部署记录**，切换到**事件**。事件出现 `OOMKilled`，表示服务的内存不够，到**设置** → **计算资源**调高**内存上限**。服务变慢或一直重新连接时，打开**用量**标签页，出现**接近内存上限**就用同样的方式调高上限。`dify` 把请求转给 `dify-api` 和 `dify-web`，所以这两个服务和 `dify-worker` 也用同样的方式检查。域名或证书一直没有完成时，见域名文档的[遇到问题](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/#遇到问题)。

不用 Zeabur 时，用 `kubectl -n <namespace> logs deployment/dify` 查看日志，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 查看事件。

详见 Zeabur 文档的[调整副本、资源和健康检查](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/service-settings/)和[查看日志、事件和用量，打开终端](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/logs-terminal/)。
