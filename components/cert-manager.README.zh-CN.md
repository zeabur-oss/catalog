## 简介

cert-manager 会向签发者申请 TLS 证书，并在到期前续期。集群要让 Zeabur 的公开域名使用 HTTPS，就需要它。

这个组件会创建两个 Let's Encrypt 的 ClusterIssuer：`letsencrypt` 和 `letsencrypt-staging`。两者都通过 Traefik 做 HTTP-01 验证，也都没有设置邮箱。建议先试 `letsencrypt-staging`：频率限制宽松得多，但浏览器不信任它签发的证书。组件还开启了 Gateway API 支持，cert-manager 可以为 Gateway 的 listener 签发证书，删除 Certificate 时也会一并删除对应的 TLS Secret。

cert-manager 会读写每个 Namespace 的 Secret，也会更新 webhook 的 CA bundle。

## 安装前准备

- 先安装 Traefik Proxy 这类入口组件。cert-manager 需要它的 Gateway API CRD。
- 做 HTTP-01 验证时，域名的 DNS 记录要指向集群，且互联网要能访问集群的 80 端口。签发者也要能访问 Let's Encrypt。
- 通配符域名（例如 `*.example.com`）需要 DNS-01 签发者，要自己创建，做法见下方的页面。

## 使用方法

在 Zeabur 的服务**网络**标签页添加公开域名。集群有多个签发者时，表单会显示**签发者**字段。详见[添加公开域名与 HTTPS](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/domains/)。同一页也说明如何为通配符域名创建 DNS-01 签发者。

## 移除

移除 cert-manager 后，就不会再签发或续期。已有的 Certificate、TLS Secret 和 CRD 都会保留，但没人续期，证书到期就失效。两个 ClusterIssuer 会被删除。账号密钥 Secret `letsencrypt-account-key` 和 `letsencrypt-staging-account-key` 会保留在 `cert-manager` Namespace。

Zeabur Builder 和 External Secrets Operator 需要它，只要其中一个还装着，Zeabur 就会拒绝移除 cert-manager。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 证书一直没有就绪

执行 `kubectl get certificate -A`，再执行 `kubectl -n <namespace> describe certificate <name>`。接着用 `kubectl get challenges -A` 检查验证挑战。HTTP-01 要求域名能从互联网访问集群的 80 端口。详见 [cert-manager 的 ACME 故障排除](https://cert-manager.io/docs/troubleshooting/acme/)。

### ClusterIssuer 没有就绪

执行 `kubectl get clusterissuer` 和 `kubectl describe clusterissuer letsencrypt`。ACME 账号注册成功，签发者才会就绪，这需要集群能访问 Let's Encrypt。
