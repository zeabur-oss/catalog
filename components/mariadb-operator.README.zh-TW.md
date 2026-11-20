## 簡介

MariaDB Operator 依 `MariaDB` 物件執行 MariaDB，可單機也可複寫。使用者、權限和資料庫也都是物件，備份會存到 S3。專案需要開源授權、與 MySQL 相容的資料庫時就裝它。Operator 採 MIT 授權，它執行的 MariaDB Server 映像檔則是 GPL-2.0。

這個元件會在 `mariadb-operator` Namespace 安裝兩個 release。一個放 CRD，讓 CRD 先於 Operator 升級；另一個放 Operator 和它的 webhook。Webhook 的憑證要靠 cert-manager 簽發。叢集還沒有 cert-manager 時，安裝對話框會先裝它。Operator 會替每個 Namespace 的 `MariaDB` 物件管理 Pod、Service、Secret、PVC、Job 和 RBAC。

## 使用方式

在 Zeabur 打開專案的**新增**選單，選 **MariaDB**。設定版本和儲存空間，再接到服務，並在資料庫的**備份**分頁設定 S3 備份。詳見[建立、備份與還原資料庫](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/)。

## 移除

移除元件會刪除 Operator、它的 webhook 和 CRD。刪除 CRD 會連帶刪除所有 `MariaDB`、備份、使用者和權限物件。有這些物件時 Zeabur 會拒絕移除，所以先刪除不要的資料庫，其餘的先備份。PVC 會留著，確定不再需要資料時，自己刪除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 資料庫一直沒有就緒

用 `kubectl -n <namespace> describe mariadb <name>` 查看物件和它的事件，再用 `kubectl -n <namespace> get pods` 列出它的 Pod。Operator 的日誌用 `kubectl -n mariadb-operator logs deploy/mariadb-operator` 查看。

### API 伺服器因 webhook 錯誤拒絕 `MariaDB` 物件

用 `kubectl -n mariadb-operator get pods` 確認 webhook 的 Pod 有在執行，再用 `kubectl -n mariadb-operator get certificate mariadb-operator-webhook-cert` 確認它的憑證已就緒。
