## 简介

Zeabur Builder 用 Dockerfile 或 Railpack，从 Git 仓库和上传的文件夹构建容器镜像，并把镜像存在 zot 里。zot 是随组件一起安装的 OCI 镜像仓库，所以构建不需要外部仓库的账号。Zeabur 开源版要从 Git 部署服务，就需要它；没有它，服务只能从镜像部署。

Builder 会为每个要构建的 Namespace 运行一个 buildkitd。组件用 cert-manager 创建一个 CA，为 zot 签发证书。推送镜像需要 Builder 签发的短期 token。NetworkPolicy 只让 Builder、buildkitd 和构建 Pod 访问 zot。zot 每个仓库保留最近的 20 个 tag，镜像存在 20Gi 的卷上。

Builder 会读取、创建、更新和删除每个 Namespace 的 Secret（Git 凭据和复制出来的凭据），并管理那里的 Build、BuildRun 和 Job。每个 buildkitd 都有特权，并搭配 user namespace 和出站规则运行。

## 安装前准备

- 先安装 cert-manager，由它签发 zot 的证书。
- 节点用输入项 `clusterIp` 的固定地址从 zot 拉镜像。这是集群 Service CIDR 里的一个空闲地址，例如 K3s 和 RKE2 上的 `10.43.0.50`。节点需要一个指向它的镜像仓库镜像（K3s 和 RKE2 写在 `registries.yaml`），并信任 `zot` Namespace 里 `zot-tls` Secret 的 CA。没有设置 `clusterIp` 时，只有 Pod 能访问仓库。

## 使用方法

在 Zeabur 开源版项目里选择**新建** → **从 Git 部署**，输入仓库。详见[从 Git 构建并部署服务](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/git/)，同一页也说明上传文件夹。构建完成的镜像由 Keel 部署，所以也要安装 Keel。

## 移除

移除 Zeabur Builder 后，所有构建都会停止。`zot` 的卷和里面所有的镜像会保留，CRD 和所有 Build 也会保留。

移除前先确认没有服务还从 zot 拉镜像。zot 移除后，重启或迁移到其他节点的 Pod 就拉不到镜像。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### zot 无法启动

zot 会等 Secret `zot-tls` 和 `zot-token-signer` 出现。执行 `kubectl -n zot get certificate,pods`。Certificate 没有就绪，就是 cert-manager 的问题。

### 服务拉不到镜像

Pod 停在 `ImagePullBackOff`。执行 `kubectl describe pod <name>` 查看事件。确认节点有指向 `clusterIp` 地址的镜像仓库镜像，并信任 `zot-tls` 的 CA。
