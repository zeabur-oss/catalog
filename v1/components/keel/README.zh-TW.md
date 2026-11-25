## 簡介

Keel 會把工作負載更新到新的映像檔 tag。它只處理帶有 `keel.sh/policy` 註解的 Deployment 等工作負載，所以裝好後不會自己改動任何東西。

Zeabur 開源版的**新建置完成後自動部署**需要它。Zeabur Builder 每次推送映像檔後，會呼叫 Keel 的 webhook（`http://keel.keel.svc:9300/v1/webhooks/native`）。對帶有 `keel.sh/trigger: poll` 註解的工作負載，Keel 也會每 5 分鐘輪詢一次映像檔倉庫，工作負載可以設定 `keel.sh/pollSchedule` 讓它更常輪詢。

元件關閉了 Helm provider，所以 Keel 不會改寫 Helm release 的 values。webhook 不需要憑證，所以 Service 是 ClusterIP，NetworkPolicy 也只讓 `builder-system` 和 `keel` 這兩個 Namespace 連到 9300 連接埠。

Keel 會讀取每個 Secret（取得映像檔倉庫的憑證）、Namespace 和 Node，並讀取、更新和刪除每個 Namespace 的 Pod、ReplicaSet、Deployment、StatefulSet、DaemonSet、Job 和 CronJob。

## 使用方式

在 Zeabur 開源版，從 Git 建置的服務只要在**建置**分頁保持開啟**新建置完成後自動部署**，就會有這個註解。詳見[新提交與自動部署](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/git/#新提交與自動部署)。

不是 Zeabur 開源版管理的工作負載，自己加上註解：

```sh
kubectl -n <namespace> annotate deployment <name> keel.sh/policy=force
```

## 移除

移除 Keel 後，就不會再自動換新映像檔。工作負載上的註解會留著，在 Keel 裝回來之前不起作用。Zeabur Builder 仍會建置，但服務不會自己換成新的映像檔。NetworkPolicy `keel-webhook` 會一起刪除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 新映像檔沒有上線

確認工作負載有註解：`kubectl -n <namespace> get deployment <name> -o jsonpath='{.metadata.annotations}'`。再用 `kubectl -n keel get pods` 確認 Keel 在執行，並用 `kubectl -n keel logs deploy/keel` 看日誌。

### 呼叫 webhook 逾時

NetworkPolicy `keel-webhook` 只允許 `builder-system` 和 `keel` Namespace 的 Pod 連入，其他 Namespace 的呼叫會直接丟掉。
