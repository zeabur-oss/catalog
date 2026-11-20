## 簡介

[OpenList](https://github.com/OpenListTeam/OpenList) 把多個儲存服務的檔案列在同一個網頁裡，也能透過 WebDAV 提供。這個範本使用一體式映像檔，內含 FFmpeg 和 Aria2。

這個範本在你的網域後面跑一個 OpenList。管理員密碼和 Aria2 密碼都在部署時自動產生。

## 開始使用

1. 在 Zeabur 打開 `openlist` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，也就是你在安裝表單填的網域。
2. 用 `admin` 登入。打開**環境變數**，在**來自設定檔和機密變數**點 `openlist-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `openlist-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

   OpenList 每次重新啟動都會重設一次這個密碼，所以在網頁裡改的密碼會失效。要改密碼，請改機密變數再重新啟動（見「設定」）。
3. 要用離線下載時，Aria2 權杖在同一個機密變數的 `aria2-password`。用同樣的方式打開，或用 kubectl 讀取：

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.aria2-password}' | base64 -d
   ```

   在 Aria2 設定裡，端點填 `http://127.0.0.1:6800/jsonrpc`，權杖填這個值。

## 設定

### 更改管理員密碼

在服務的**環境變數**分頁點 `openlist-secrets`，填入新的 `admin-password`，檢查後套用。再按服務頁上方的**重新啟動**，讓 OpenList 讀到新值。

### 使用離線下載

開啟管理頁面，進入設定，再進入其他，設好暫存資料夾。接著填入「開始使用」取得的 Aria2 端點和權杖。詳見 [Offline download](https://doc.oplist.org/guide/advanced/offline-download)。

### 用密碼保護資料夾

為路徑新增一筆 meta 記錄，並設定密碼。密碼要涵蓋子資料夾時，勾選密碼後面的 Apply to sub folder。透過 WebDAV 存取時，這個密碼不會生效。詳見 [Meta](https://doc.oplist.org/guide/advanced/meta)。
