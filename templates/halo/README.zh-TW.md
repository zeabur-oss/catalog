## 簡介

[Halo](https://github.com/halo-dev/halo) 是網站與部落格系統，有主題和外掛。這個範本在你的網域後面跑 Halo，搭配 PostgreSQL 資料庫。

## 開始使用

1. 在 Zeabur 打開 `halo` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `halo.example.com`。訪客在這裡看到網站，管理後台在 `/console`。
2. 第一次啟動時會建立超級管理員 `admin`，不會出現初始化精靈。取得它的密碼。打開**環境變數**，在**來自設定檔和機密變數**點 `halo-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `halo-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret halo-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 開啟 `https://halo.example.com/console`，用 `admin` 和這組密碼登入，再到後台更改密碼。

## 設定

### 安裝主題

在後台開啟「主題」，再點右上角的「主題管理」。你可以從內建的應用市場安裝主題、上傳主題壓縮檔，或用網址下載。安裝後選取主題，再點「啟用」。詳見 [主題](https://docs.halo.run/guide/use/themes)。

### 備份網站

在後台開啟「備份」，點「建立備份」。Halo 會在背景進行備份。備份檔放在 Halo 工作目錄的 `backups` 資料夾，也就是磁碟區 `halo-data` 上。請另外在別處保留一份。詳見 [備份與恢復](https://docs.halo.run/guide/use/backup)。
