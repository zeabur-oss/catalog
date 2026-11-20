## 简介

System Upgrade Controller 按照 Plan 升级节点上的 K3s 或 RKE2。Plan 指定节点和目标版本，控制器会一次升级一个节点（或按 `concurrency` 允许的数量），升级时先 cordon 并 drain 节点，再在每个节点上运行有特权的 Job。

这个组件是可选的。只在 Plan 不改变 Kubernetes 版本（例如升级操作系统软件包），或没有其他工具替你升级节点时才需要装。节点上如果有工具固定了 K3s 或 RKE2 的版本（例如 Zeabur 的安装程序），就不要装，那个工具会把版本改回固定的版本，覆盖 Plan 的升级。Plan 会替换 K3s 或 RKE2 的可执行文件，所以组件只支持 K3s 和 RKE2。

控制器会 cordon 并 drain 节点，Job 有特权，并在主机上运行 Plan 指定的镜像。

## 使用方法

Zeabur 没有这个组件的界面。在 `system-upgrade` Namespace 创建 Plan。下面的 Plan 把 K3s 集群的 server 节点升级到你指定的版本：

```yaml
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: server-plan
  namespace: system-upgrade
spec:
  concurrency: 1
  cordon: true
  nodeSelector:
    matchExpressions:
      - key: node-role.kubernetes.io/control-plane
        operator: In
        values: ["true"]
  serviceAccountName: system-upgrade
  upgrade:
    image: rancher/k3s-upgrade
  version: <K3s 版本，例如 v1.33.4+k3s1>
```

先升级 server 节点，再升级 agent 节点，也不要跳过次要版本。agent 的 Plan 和 RKE2（`rancher/rke2-upgrade`）见 [K3s 自动升级](https://docs.k3s.io/upgrades/automated)和 [RKE2 自动升级](https://docs.rke2.io/upgrades/automated_upgrade)。用 `kubectl -n system-upgrade get plans,jobs` 查看进度。

## 移除

移除控制器后，所有 Plan 都会停止。已被 Plan 升级的节点会保持新版本。移除前先确认没有升级 Job 在运行：`kubectl -n system-upgrade get jobs`。详见[移除组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/#移除组件)。

## 故障排除

### Plan 没有任何动作

Plan 要创建在 `system-upgrade` Namespace，也就是控制器所在的 Namespace。执行 `kubectl -n system-upgrade get plans -o wide`，并用 `kubectl -n system-upgrade logs deploy/system-upgrade-controller` 查看控制器的日志。用 `kubectl get nodes --show-labels` 确认 `nodeSelector` 与你的节点匹配。
