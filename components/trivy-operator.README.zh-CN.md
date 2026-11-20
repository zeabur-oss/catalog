## 简介

Trivy Operator 扫描运行中工作负载的镜像，找出已知漏洞。它也会检查工作负载的配置，并找出泄露的机密。结果写成报告，放在该工作负载所在的 Namespace：`VulnerabilityReport`、`ConfigAuditReport` 和 `ExposedSecretReport`。

想知道集群里哪些镜像有漏洞时，就安装它。扫描 Job 会占用不少 CPU 和内存，所以这个组件是可选的。这个组件只列出有修复版本的漏洞。单节点时，一次扫描一个镜像，只报告 MEDIUM、HIGH 和 CRITICAL 的结果，并跳过 RBAC 检查。

Operator 读取每个 Namespace 的工作负载、Pod、Service、Ingress 和 RBAC 对象，运行扫描 Job 并写入报告，不会读取所有 Secret。基础设施扫描、合规扫描和 SBOM 报告都是关闭的。

## 使用方法

Trivy Operator 没有 Zeabur 界面。等第一批扫描完成后，用 kubectl 读取报告。

```sh
kubectl get vulnerabilityreports -A -o wide
kubectl get configauditreports -A -o wide
```

报告字段和设置详见 [Trivy Operator 文档](https://aquasecurity.github.io/trivy-operator/)。

## 移除

移除会删除 operator。CRD 会保留，已经写好的报告也会保留，不想要时自己删除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 没有出现报告

查看 operator 的日志，并找出失败的扫描 Job：

```sh
kubectl -n trivy-system logs deploy/trivy-operator
kubectl -n trivy-system get jobs,pods
```
