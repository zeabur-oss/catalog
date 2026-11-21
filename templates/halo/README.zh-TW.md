## 簡介

[Halo](https://github.com/halo-dev/halo) 是網站與部落格系統，有主題和外掛。這個範本在你的網域後面跑 Halo，搭配 PostgreSQL 資料庫。

## 開始使用

1. 在 Zeabur 開源版打開 `halo` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `halo.example.com`。訪客在這裡看到網站，管理後台在 `/console`。
2. 第一次啟動時會建立超級管理員 `admin`，不會出現初始化精靈。取得它的密碼。打開**環境變數**，在**來自設定檔和機密變數**點 `halo-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `halo-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret halo-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 開啟 `https://halo.example.com/console`，用 `admin` 和這組密碼登入，再到後台更改密碼。

## 設定

### 安裝主題

在後台開啟「主題」，再點右上角的「主題管理」。你可以從內建的應用市場安裝主題、上傳主題壓縮檔，或用網址下載。安裝後選取主題，再點「啟用」。詳見 [主題](https://docs.halo.run/guide/use/themes)。

### 備份資料

Halo 有內建的備份。在後台開啟「備份」，點「建立備份」。Halo 會在背景進行備份。備份檔放在 Halo 工作目錄的 `backups` 資料夾，也就是磁碟區 `halo-data`（`/var/lib/halo`）上。請另外在別處保留一份。

Halo 把內容存在 `halo-db` 資料庫。在專案打開 `halo-db`，到**備份**設定 S3 備份。想直接拿到檔案，按**更多** → **匯出**。

很大的備份檔在瀏覽器裡可能上傳失敗。改用 kubectl 複製到 `backups` 資料夾，再從後台的備份檔還原：

```sh
kubectl -n <namespace> cp ./<backup file> halo-0:/var/lib/halo/backups/
```

詳見 Halo 文件的[備份與恢復](https://docs.halo.run/guide/use/backup)。詳見 Zeabur 開源版文件的[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)和[下載資料庫傾印檔](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#下載資料庫傾印檔)。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `halo` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 開源版時，用 `kubectl -n <namespace> logs statefulset/halo` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 開源版文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。
