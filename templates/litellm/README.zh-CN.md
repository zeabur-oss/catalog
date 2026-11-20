## 简介

[LiteLLM](https://github.com/BerriAI/litellm) 是代理，在多家 LLM 服务前面提供一个兼容 OpenAI 的 API。它也有管理页面，用来管理模型、密钥和花费。

这个模板在你的域名后面运行 LiteLLM，模型、密钥和花费都存在 PostgreSQL。

## 开始使用

1. 在 Zeabur 打开 `litellm` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，例如 `litellm.example.com`。管理页面在这个域名的 `/ui`。
2. 获取主密钥。打开**变量**，在**来自配置文件和机密变量**中点击 `litellm-secrets`，再点击 `master-key` 旁的眼睛图标。不用 Zeabur 时，用 kubectl 读取机密变量 `litellm-secrets` 的 `master-key`：

   ```sh
   kubectl -n <namespace> get secret litellm-secrets -o jsonpath='{.data.master-key}' | base64 -d
   ```

   实际使用的密钥是 `sk-` 加上这个值。
3. 用 `admin` 登录，密码是 `sk-` 加上刚才读到的值。
4. 同一把 `sk-` 密钥也是调用 `https://litellm.example.com` 的管理员 API 密钥。模型和给客户端的密钥都在管理页面里添加。

`litellm-secrets` 的 `salt-key` 用来加密数据库里模型服务的凭证。添加模型后，请不要再修改它。

## 配置

### 添加模型

在管理页面打开 Models + Endpoints，选择模型服务并填入它的 API 密钥。在页面添加的模型存在数据库里，不用编辑文件，也不用重启。详见 [Model Management](https://docs.litellm.ai/docs/proxy/model_management)。

### 为客户端创建密钥

请不要把主密钥交给应用。在管理页面为每个客户端创建虚拟密钥，再把客户端指向 `https://litellm.example.com/v1` 并使用这把密钥。详见 [Virtual Keys](https://docs.litellm.ai/docs/proxy/virtual_keys) 和 [Langchain, OpenAI SDK 等客户端](https://docs.litellm.ai/docs/proxy/user_keys)。
