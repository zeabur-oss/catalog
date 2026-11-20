## 简介

Keel 会把工作负载更新到新的镜像 tag。它只处理带有 `keel.sh/policy` 注解的 Deployment 等工作负载，所以装好后不会自己改动任何东西。

Zeabur 的**新构建完成后自动部署**需要它。Zeabur Builder 每次推送镜像后，会调用 Keel 的 webhook（`http://keel.keel.svc:9300/v1/webhooks/native`）。对带有 `keel.sh/trigger: poll` 注解的工作负载，Keel 也会每 5 分钟轮询一次镜像仓库，工作负载可以设置 `keel.sh/pollSchedule` 让它更频繁地轮询。

组件关闭了 Helm provider，所以 Keel 不会改写 Helm release 的 values。webhook 不需要凭据，所以 Service 是 ClusterIP，NetworkPolicy 也只让 `builder-system` 和 `keel` 这两个 Namespace 访问 9300 端口。

Keel 会读取每个 Secret（获取镜像仓库的凭据）、Namespace 和 Node，并读取、更新和删除每个 Namespace 的 Pod、ReplicaSet、Deployment、StatefulSet、DaemonSet、Job 和 CronJob。

## 使用方法

在 Zeabur，从 Git 构建的服务只要在**构建**标签页保持开启**新构建完成后自动部署**，就会有这个注解。详见[新提交与自动部署](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/git/#新提交与自动部署)。

不是 Zeabur 管理的工作负载，自己加上注解：

```sh
kubectl -n <namespace> annotate deployment <name> keel.sh/policy=force
```

## 移除

移除 Keel 后，就不会再自动换新镜像。工作负载上的注解会保留，在 Keel 装回来之前不起作用。Zeabur Builder 仍会构建，但服务不会自己换成新的镜像。NetworkPolicy `keel-webhook` 会一起删除。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 新镜像没有上线

确认工作负载有注解：`kubectl -n <namespace> get deployment <name> -o jsonpath='{.metadata.annotations}'`。再用 `kubectl -n keel get pods` 确认 Keel 在运行，并用 `kubectl -n keel logs deploy/keel` 查看日志。

### 调用 webhook 超时

NetworkPolicy `keel-webhook` 只允许 `builder-system` 和 `keel` Namespace 的 Pod 访问，其他 Namespace 的调用会直接丢掉。
