## 簡介

Longhorn 是區塊儲存，會把每個磁碟區複製到其他節點。節點故障時磁碟區仍在，會跟著 Pod 換到別的節點，也能原地擴充。多節點叢集建議安裝，它會成為預設的儲存類別。單節點叢集可選裝：副本沒有其他節點可去，但仍能用到可擴充的磁碟區和快照。KubeSolo 不能裝。

這個元件讓每個磁碟區最多有三份副本，每個節點放一份，有幾個節點就放幾份。節點比副本少時，磁碟區照樣能掛載，節點加入後再補上缺少的副本。單節點時，預設儲存類別仍是 `local-path`，`longhorn` 類別只要一個副本。

Longhorn 在每個節點執行特權 Pod。這些 Pod 會從主機掛載 `/dev`、`/proc`、`/boot`、`/etc` 和 `/var/lib/longhorn`，並管理整個叢集的 Pod、PersistentVolume 和儲存類別。Longhorn UI 沒有登入驗證，也沒有 Ingress，而且 NetworkPolicy 不讓其他 Pod 連到。

## 安裝前準備

先把每個節點準備好：

- open-iscsi，`iscsid` 要在執行，`iscsi_tcp` 模組要載入。
- NFSv4 用戶端，供 ReadWriteMany 磁碟區和 NFS 備份使用。
- `bash`、`curl`、`findmnt`、`grep`、`awk`、`blkid` 和 `lsblk`。
- 如果有執行 `multipathd`，要把 Longhorn 的磁碟列入黑名單，見 [Longhorn 的 multipath 說明](https://longhorn.io/kb/troubleshooting-volume-with-multipath/)。

在節點上執行 `longhornctl check preflight` 可以檢查。安裝後，`kubectl -n longhorn-system get nodes.longhorn.io` 也會在各節點的狀況顯示同樣的結果。

## 使用方式

在 Zeabur 開源版新增磁碟區時，在**儲存類別**選**複寫磁碟**。詳見[選擇儲存類別](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/volumes/#選擇儲存類別)。在 Zeabur 開源版之外，把 PVC 的 `storageClassName` 設為 `longhorn`：

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata: {name: data, namespace: <namespace>}
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: longhorn
  resources:
    requests: {storage: 1Gi}
```

要開啟 Longhorn UI，執行 `kubectl -n longhorn-system port-forward svc/longhorn-frontend 8080:80`，再瀏覽 `http://localhost:8080`。詳見 [Longhorn 文件](https://longhorn.io/docs/1.13.0/)。

## 移除

移除 Longhorn 會刪除所有磁碟區和裡面的資料。先備份，並讓所有工作負載離開 Longhorn 磁碟區。Longhorn 要先確認才會解除安裝：

```sh
kubectl -n longhorn-system patch -p '{"value": "true"}' --type=merge lhs deleting-confirmation-flag
```

`longhorn-static` 儲存類別會留著，因為 Longhorn 建立它時沒有設擁有者，要自己刪除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 節點不是 Ready

執行 `kubectl -n longhorn-system get nodes.longhorn.io`。節點的 Ready 狀況會寫出它缺什麼，例如 open-iscsi、NFS 用戶端或核心模組。到那個節點上補裝缺少的項目。

### 磁碟區一直是降級狀態，或掛載不上

用 `kubectl -n longhorn-system get pods` 檢查 Pod，再看 `kubectl -n longhorn-system get volumes.longhorn.io`。節點比副本少的叢集，在更多節點加入前，磁碟區會用較少的副本執行。
