## 簡介

Dragonfly Operator 依 `Dragonfly` 物件執行 Dragonfly（相容 Redis 的記憶體資料庫）：一個主節點加副本、故障切換，以及快照到磁碟區。專案需要相容 Redis 的資料庫時就裝它。

Operator 採 Apache-2.0 授權。它執行的 Dragonfly 映像檔採 BUSL-1.1，屬於原始碼可得（source-available）授權，不是 OSI 認可的開源授權。使用前請先閱讀授權的限制，尤其是向他人提供託管資料庫服務的情境。

Operator 會替每個 Namespace 的 `Dragonfly` 物件管理 Pod、StatefulSet、Service 和 PodDisruptionBudget。`kube-rbac-proxy` 容器負責保護它的指標連接埠。

## 使用方式

在 Zeabur 打開專案的**新增**選單，選 **Dragonfly**。設定儲存空間，再接到服務。Dragonfly 沒有**備份**分頁，快照存在自己的磁碟區，啟動時載入最新的一份。詳見[建立、備份與還原資料庫](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/)。

## 移除

移除 Operator 後，CRD 和所有 `Dragonfly` 物件都會留著。在 Operator 裝回來之前，沒有人管理這些物件。只要還有 `Dragonfly` 物件，Zeabur 就拒絕移除這個元件。請先刪除不要的資料庫。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 資料庫一直沒有就緒

用 `kubectl -n <namespace> get dragonfly <name>` 查看物件，再用 `kubectl -n <namespace> get pods` 查看它的 Pod。Operator 的日誌用 `kubectl -n dragonfly-operator-system logs deploy/dragonfly-operator -c manager` 查看。
