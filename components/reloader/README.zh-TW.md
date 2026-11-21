## 簡介

Stakater Reloader 會在 Deployment、StatefulSet 或 DaemonSet 使用的 ConfigMap 或 Secret 變更時，重新啟動這個工作負載。它只重新啟動帶有 `reloader.stakater.com/auto: "true"` 或 `reloader.stakater.com/reload` 註解的工作負載，所以裝好後不會自己重新啟動任何服務。

想讓服務在共享變數或 Secret 改變後自動套用新值、不用手動重新啟動時，就裝它。Reloader 會讀取叢集裡所有的 ConfigMap 和 Secret，也能修改每個 Namespace 的 Deployment、StatefulSet 和 DaemonSet。Job 和 CronJob 不受影響。

## 使用方式

在 Zeabur 開源版打開服務的**設定** → **運算資源**，開啟**設定檔或機密變數變更時自動重新啟動**，再檢查並套用。Zeabur 開源版會幫你加上註解。詳見[變數](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/variables/)。

不是 Zeabur 開源版管理的工作負載，自己加上註解：

```sh
kubectl -n <namespace> annotate deployment <name> reloader.stakater.com/auto=true
```

## 移除

移除 Reloader 後，就不會再自動重新啟動。工作負載上的註解會留著，在 Reloader 裝回來之前不起作用。移除後，ConfigMap 或 Secret 改了，要自己重新啟動服務。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 服務還在用舊的值

確認服務開了這個選項，或工作負載上有註解（`kubectl -n <namespace> get deployment <name> -o jsonpath='{.metadata.annotations}'`）。再確認 Reloader 有在執行：`kubectl -n reloader get pods`，並用 `kubectl -n reloader logs deploy/reloader-reloader` 看日誌。
