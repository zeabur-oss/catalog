## 簡介

SeaweedFS Operator 依 `Seaweed` 物件執行 SeaweedFS 物件儲存，並提供 S3 API。儲存貯體、使用者、金鑰和政策也都是物件。專案需要叢集內的 S3 物件儲存時就裝它。

這個元件目前為 Beta。chart 大多出自單一維護者，功能和 CRD 在版本之間仍會變動。Operator 的 webhook 憑證要靠 cert-manager 簽發。

Operator 會替每個 Namespace 的 `Seaweed` 物件管理 Pod、StatefulSet、Service、Secret、PVC、Job 和 Ingress，也會呼叫各儲存服務的管理 API。Grafana 儀表板是關閉的。

## 使用方式

在 Zeabur 開源版打開專案的**新增**選單，選**物件儲存**。設定容量和第一個儲存貯體，再把 S3 端點和金鑰接到服務。詳見[建立 S3 物件儲存與儲存貯體](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/object-storage/)。

## 移除

移除 Operator 只會刪除 Operator 本身和 webhook。CRD 會留著，你的 `Seaweed` 和儲存貯體物件、Operator 建立的 Pod 和它們的 PVC 也都會留著。在 Operator 裝回來之前，沒有人管理這些物件。只要還有 `Seaweed` 物件，Zeabur 開源版就拒絕移除這個元件。請先在 Zeabur 開源版刪除物件儲存，還需要的檔案先備份。已刪除儲存服務留下的 PVC 要自己刪除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 儲存服務一直沒有就緒

Operator 會在 master、volume 和 filer 之後才建立 S3 閘道。用 `kubectl -n <namespace> get seaweed` 和 `kubectl -n <namespace> get pods` 看進度，再用 `kubectl -n seaweedfs-operator logs deploy/seaweedfs-operator` 看 Operator 的日誌。

### `Seaweed` 物件被 webhook 拒絕

Webhook 需要憑證。用 `kubectl -n seaweedfs-operator get certificate seaweedfs-operator-webhook-cert` 檢查，並確認 cert-manager 在執行。
