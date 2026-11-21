## 簡介

CloudNativePG 依 `Cluster` 物件執行 PostgreSQL，支援副本、故障切換和備份到 S3。專案需要 PostgreSQL 資料庫時就裝它。這個元件會在 `cnpg-system` Namespace 安裝兩個 release：Operator，以及負責寫入備份的 Barman Cloud 外掛。

Operator 需要 cert-manager，用憑證保護 Operator 和外掛之間的連線。叢集還沒有 cert-manager 時，安裝對話框會先裝它。Operator 會替每個 Namespace 的 `Cluster` 物件管理 Pod、Service、Secret 和 PVC。PodMonitor 和 Grafana 儀表板是關閉的。

## 使用方式

在 Zeabur 開源版打開專案的**新增**選單，選 **PostgreSQL**。設定版本和儲存空間，再接到服務，並在資料庫的**備份**分頁設定 S3 備份。詳見[建立、備份與還原資料庫](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/)。

CloudNativePG 會把備份檔以明文寫進儲存貯體。請用開啟伺服器端加密的儲存貯體，並限制誰能讀取。

## 移除

移除元件會移除 Operator 和外掛。只要還有 `Cluster` 物件，Zeabur 開源版就拒絕移除，所以先刪除不要的資料庫，其餘的先備份。CRD 會留著，已刪除資料庫的 PVC 也會留著。確定不再需要資料時，自己刪除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 資料庫一直沒有就緒

用 `kubectl -n <namespace> describe cluster <name>` 查看物件和它的事件，再用 `kubectl -n <namespace> get pods` 列出它的 Pod。Operator 的日誌用 `kubectl -n cnpg-system logs deploy/cnpg-cloudnative-pg` 查看。

### 備份沒有開始

用 `kubectl -n cnpg-system get pods` 確認外掛有在執行，再用 `kubectl -n cnpg-system get certificate` 確認它的憑證已就緒。外掛需要 `barman-cloud-client` 和 `barman-cloud-server` 兩張憑證。
