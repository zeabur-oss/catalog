## 簡介

MOCO 依 `MySQLCluster` 物件執行 MySQL，可以是單一執行個體，也可以是半同步的主節點加副本。MOCO 會備份到 S3，並能還原到某個時間點。應用程式需要 MySQL 本身時就裝它。Operator 採 Apache-2.0 授權，執行的 MySQL 映像檔則是 GPL-2.0。

MOCO 需要 cert-manager 簽發憑證，用於 webhook，以及控制器與代理程式之間的 mTLS。叢集還沒有 cert-manager 時，安裝對話框會先裝它。控制器會替每個 Namespace 的 `MySQLCluster` 物件管理 Pod、StatefulSet、Service、Secret、PVC、CronJob、Job 和 Role，並把憑證複製進去。

## 使用方式

在 Zeabur 打開專案的**新增**選單，選 **MySQL**。設定版本和儲存空間，再接到服務，並在資料庫的**備份**分頁設定 S3 備份。詳見[建立、備份與還原資料庫](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/)。

## 移除

移除 MOCO 會移除控制器和它的 webhook。只要還有 `MySQLCluster` 物件，Zeabur 就拒絕移除，所以先刪除不要的資料庫，其餘的先備份。CRD 會留著，已刪除資料庫的 PVC 也會留著。確定不再需要資料時，自己刪除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 資料庫一直不是 Healthy

用 `kubectl -n <namespace> get mysqlcluster <name>` 查看物件，再用 `kubectl -n <namespace> get pods` 查看它的 Pod。控制器的日誌用 `kubectl -n moco-system logs deploy/moco-controller` 查看。

### API 伺服器因 webhook 錯誤拒絕 `MySQLCluster` 物件

用 `kubectl -n moco-system get pods` 確認控制器有在執行，再用 `kubectl -n moco-system get certificate` 確認它的憑證已就緒。
