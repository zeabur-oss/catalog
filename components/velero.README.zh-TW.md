## 簡介

Velero 把 Kubernetes 物件和磁碟區裡的檔案備份到 S3 相容儲存，也能還原。想備份、還原整個專案時就裝它。資料庫備份不需要它。

Velero 綁定 `cluster-admin`，能讀取並還原叢集裡所有的物件，所以只給管理員使用。預設沒有任何 Pod 是特權模式。CSI 快照是關閉的。Velero 用 Kopia 複製磁碟區裡的檔案，要開啟 `fileBackup` 輸入才會複製。Velero 會把 Secret 原樣存進備份，所以安裝時會產生兩把金鑰。一把是 Kopia 儲存庫密碼，另一把讓 S3 伺服器加密每個備份物件（SSE-C）。

## 安裝前準備

先準備一個叢集外的 S3 儲存貯體。備份放在叢集裡的話，叢集一壞，備份也跟著沒了。接著備齊這些輸入：

- `bucket`：儲存貯體名稱。
- `s3Url`：S3 端點網址。要用 HTTPS，儲存服務也要支援 SSE-C。
- `region`：儲存貯體所在區域，預設為 `us-east-1`。
- `credentials`：儲存貯體的金鑰，格式是 AWS 風格的憑證檔，在 `[default]` 設定檔裡寫 `aws_access_key_id` 和 `aws_secret_access_key`。
- `fileBackup`：不需要備份磁碟區檔案就維持 `false`。設為 `true` 時，每個節點會以 root 執行一個節點代理程式，掛載主機路徑並持有 Velero 的權杖，`velero` Namespace 也必須允許特權（privileged）Pod 安全等級。

## 使用方式

在 Zeabur 打開**叢集** → **備份**，設定備份存放位置和排程、執行備份與還原。詳見[備份叢集並從備份還原](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/backups/)。

這兩把金鑰也存在它們保護的備份裡。現在就複製到叢集外。沒有金鑰，就沒有人能讀取備份。

```sh
kubectl -n velero get secret velero-repo-credentials velero-backup-key -o yaml > velero-keys.yaml
```

## 移除

移除 Velero 會刪除它的 Deployment，以及你開啟過的節點代理程式。備份和還原會停止。CRD，以及 Backup、Schedule、Restore 物件會留在叢集裡。儲存貯體裡的備份和產生的金鑰也都會留著。請保管好金鑰，讀取舊備份時要用。移除前先讓執行中的備份跑完。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 備份存放位置連不上

用 `kubectl -n velero get backupstoragelocations` 和 `kubectl -n velero describe backupstoragelocation default` 看它的狀態和訊息。再檢查儲存貯體、端點、區域和金鑰。Velero 的日誌用 `kubectl -n velero logs deploy/velero` 查看。

### 備份結果是 PartiallyFailed

`fileBackup` 為 `false` 時，要求複製磁碟區檔案的備份會因為複製不了而部分失敗。執行 `kubectl -n velero get backups`，再用 `velero backup describe <backup> --details` 查看失敗的項目。把元件的 `fileBackup` 設為 `true`，再備份一次。
