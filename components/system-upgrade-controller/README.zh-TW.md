## 簡介

System Upgrade Controller 依照 Plan 升級節點上的 K3s 或 RKE2。Plan 指定節點和目標版本，控制器會一次升級一個節點（或依 `concurrency` 允許的數量），升級時先 cordon 並 drain 節點，再在每個節點上執行有特權的 Job。

這個元件是選用的。只在 Plan 不改變 Kubernetes 版本（例如升級作業系統套件），或沒有其他工具替你升級節點時才需要裝。節點上若有工具固定了 K3s 或 RKE2 的版本（例如 Zeabur 開源版的安裝程式），就不要裝，那個工具會把版本改回固定的版本，蓋掉 Plan 的升級。Plan 會替換 K3s 或 RKE2 的執行檔，所以元件只支援 K3s 和 RKE2。

控制器會 cordon 並 drain 節點，Job 有特權，並在主機上執行 Plan 指定的映像檔。

## 使用方式

Zeabur 開源版沒有這個元件的畫面。在 `system-upgrade` Namespace 建立 Plan。下面的 Plan 把 K3s 叢集的 server 節點升級到你指定的版本：

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

先升級 server 節點，再升級 agent 節點，也不要跳過次要版本。agent 的 Plan 和 RKE2（`rancher/rke2-upgrade`）見 [K3s 自動升級](https://docs.k3s.io/upgrades/automated)和 [RKE2 自動升級](https://docs.rke2.io/upgrades/automated_upgrade)。用 `kubectl -n system-upgrade get plans,jobs` 看進度。

## 移除

移除控制器後，所有 Plan 都會停止。已被 Plan 升級的節點會保持新版本。移除前先確認沒有升級 Job 在執行：`kubectl -n system-upgrade get jobs`。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### Plan 沒有任何動作

Plan 要建立在 `system-upgrade` Namespace，也就是控制器所在的 Namespace。執行 `kubectl -n system-upgrade get plans -o wide`，並用 `kubectl -n system-upgrade logs deploy/system-upgrade-controller` 看控制器的日誌。用 `kubectl get nodes --show-labels` 確認 `nodeSelector` 符合你的節點。
