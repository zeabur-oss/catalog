## 簡介

Percona Operator for MongoDB 依 `PerconaServerMongoDB` 物件執行 MongoDB 複本集，並用 Percona Backup for MongoDB 排程備份到 S3。專案需要 MongoDB 時就裝它。

Operator 採 Apache-2.0 授權。它執行的 Percona Server for MongoDB 映像檔採 SSPL-1.0，這不是 OSI 認可的開源授權。使用前請先閱讀 SSPL。

Operator 會替每個 Namespace 的 MongoDB 叢集管理 Pod、Service、Secret 和 PVC。這個元件關閉了 Operator 的遙測（預設會回報給 Percona）。

## 安裝前準備

Operator 映像檔以 UBI 10 為基底，在 x86 節點上需要支援 x86-64-v3 的 CPU。每個資料庫都用這個映像檔當 init 容器，所以在舊 CPU 上 Operator 能啟動，第一個資料庫卻起不來。它的第一個 Pod（例如 `mongodb-rs0-0`）會停在 `Init:CrashLoopBackOff`，init 容器的日誌會出現 `Fatal glibc error: CPU does not support x86-64-v3`。

每個可能執行資料庫的節點都要檢查。下面的指令必須印出全部四個旗標：

```sh
grep -o -w -E 'avx2|bmi2|fma|movbe' /proc/cpuinfo | sort -u
```

## 使用方式

在 Zeabur 開源版打開專案的**新增**選單，選 **MongoDB**。設定版本和儲存空間，再接到服務，並在資料庫的**備份**分頁設定 S3 備份。詳見[建立、備份與還原資料庫](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/)。

## 移除

移除 Operator 後，CRD 會留著。只要還有 `PerconaServerMongoDB` 物件，Zeabur 開源版就拒絕移除，所以先刪除不要的資料庫，其餘的先備份。已刪除資料庫的 PVC 會留著。確定不再需要資料時，自己刪除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 資料庫一直沒有就緒

用 `kubectl -n <namespace> get psmdb <name>` 查看物件，再用 `kubectl -n <namespace> get pods` 查看它的 Pod。Operator 的日誌用 `kubectl -n psmdb-operator logs deploy/psmdb-operator` 查看。

### MongoDB 8 的 Pod 一啟動就當掉

MongoDB 8 無法在 Linux 6.19 以上啟動（SERVER-121912）。用 `uname -r` 查看節點的核心版本。Zeabur 開源版建立的 MongoDB 8 資料庫都會加上 `GLIBC_TUNABLES=glibc.pthread.rseq=1`。自己建立的資料庫請改用 7.0 映像檔。
