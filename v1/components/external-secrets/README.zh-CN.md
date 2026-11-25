## 简介

External Secrets Operator 会把外部机密库（例如 OpenBao、Vault、1Password 或 AWS Secrets Manager）中的机密复制为 Kubernetes Secret，并按你设置的间隔保持同步。服务读取这个 Secret 的方式和普通 Secret 相同。

这个组件是可选的。添加 SecretStore 之前，它不会做任何事。webhook 的证书由 cert-manager 签发，所以 Chart 自己的证书控制器是关闭的。

Operator 会读取、创建、更新和删除每个 Namespace 的 Secret，也会为每个 Namespace 创建 ServiceAccount token，让 SecretStore 能以 ServiceAccount 身份登录。

## 安装前准备

- 先安装 cert-manager，由它签发 webhook 的证书。
- 准备好机密库，以及只读的凭据。

## 使用方法

在服务所在的 Namespace 创建 SecretStore，再创建 ExternalSecret 指定要复制的值。详见[从外部机密库同步机密变量](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/external-secrets/)，里面有 OpenBao、Vault、1Password 和 AWS Secrets Manager 的步骤。

## 移除

移除 Chart 时，CRD 也会删除，所有 SecretStore 和 ExternalSecret 会随之消失。在默认的 `creationPolicy: Owner` 下，ExternalSecret 拥有自己的 Secret，Secret 也会删除，服务就读不到值了。移除前，把必须保留 Secret 的 ExternalSecret 设为 `creationPolicy: Orphan`，或先复制这些 Secret。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### ExternalSecret 没有就绪

执行 `kubectl -n <namespace> describe externalsecret <name>`，状态会告诉你是不是 SecretStore 的问题。再执行 `kubectl -n <namespace> describe secretstore <name>`，确认它能连到机密库。

### 创建 SecretStore 失败

写入 SecretStore 时，请求会经过 webhook。用 `kubectl -n external-secrets get pods` 和 `kubectl -n external-secrets get certificate` 检查。
