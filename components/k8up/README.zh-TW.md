## 簡介

K8up 用 restic 把 PersistentVolumeClaim 的內容備份到 S3 相容儲存。服務需要備份自己的磁碟區時就裝它。K8up 備份的是單一服務的檔案，不是整個叢集，資料庫備份不需要它。

K8up 只備份帶有 `k8up.io/backup=true` 註解的 PVC，所以裝好後不會自己備份任何東西。控制器會監看 K8up 物件，並讀取每個 Namespace 的 PVC、PersistentVolume 和 Pod，也會在磁碟區所在的 Namespace 建立備份 Job、ServiceAccount 和 RoleBinding。每個備份 Job 只掛載自己 Namespace 裡的磁碟區。

## 使用方式

在 Zeabur 打開服務的**設定** → **儲存空間**，在下方的**備份**找到磁碟區，按**備份這個磁碟區**。先備好 S3 儲存貯體和金鑰，或請管理員登錄 S3 目的地。Zeabur 會建立 K8up Schedule，以及存放 restic 密碼的 Secret。詳見[設定磁碟區備份](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/volumes/#設定磁碟區備份)。

把備份 Secret 裡的 `RESTIC_PASSWORD` 另外存到安全的地方。沒有它，就讀不了 S3 裡的備份。

## 移除

移除 K8up 後，所有排程備份都會停止。chart 被移除時，清理 Job 會刪除 K8up 在每個 Namespace 加的 `pod-executor` Role 和 RoleBinding。CRD 會留著，Schedule 物件、備份 Secret 和 S3 儲存貯體裡的資料也都會留著。在 K8up 裝回來之前，Schedule 不會做任何事。要永久停止某個備份，用 `kubectl -n <namespace> delete schedule.k8up.io <name>` 刪除排程。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 備份沒有開始或失敗

確認 PVC 帶有 `k8up.io/backup=true`：`kubectl -n <namespace> get pvc <name> -o jsonpath='{.metadata.annotations}'`。再用 `kubectl -n <namespace> get backups.k8up.io` 列出執行紀錄，並用 `kubectl -n k8up-system logs deploy/k8up` 看 operator 的日誌。
