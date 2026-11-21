## 简介

Stakater Reloader 会在 Deployment、StatefulSet 或 DaemonSet 使用的 ConfigMap 或 Secret 变更时，重启这个工作负载。它只重启带有 `reloader.stakater.com/auto: "true"` 或 `reloader.stakater.com/reload` 注解的工作负载，所以装好后不会自己重启任何服务。

想让服务在共享变量或 Secret 变化后自动使用新值、不用手动重启时，就装它。Reloader 会读取集群里所有的 ConfigMap 和 Secret，也能修改每个 Namespace 的 Deployment、StatefulSet 和 DaemonSet。Job 和 CronJob 不受影响。

## 使用方法

在 Zeabur 开源版打开服务的**设置** → **计算资源**，开启**配置文件或机密变量变更时自动重启**，再检查并应用。Zeabur 开源版会帮你加上注解。详见[环境变量](https://oss.zeabur.com/zh-CN/docs/zeabur/configure/variables/)。

不是 Zeabur 开源版管理的工作负载，自己加上注解：

```sh
kubectl -n <namespace> annotate deployment <name> reloader.stakater.com/auto=true
```

## 移除

移除 Reloader 后，就不会再自动重启。工作负载上的注解会保留，在 Reloader 装回来之前不起作用。移除后，ConfigMap 或 Secret 改了，要自己重启服务。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### 服务还在用旧的值

确认服务开启了这个选项，或工作负载上有注解（`kubectl -n <namespace> get deployment <name> -o jsonpath='{.metadata.annotations}'`）。再确认 Reloader 正在运行：`kubectl -n reloader get pods`，并用 `kubectl -n reloader logs deploy/reloader-reloader` 查看日志。
