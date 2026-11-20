## 簡介

[OpenList](https://github.com/OpenListTeam/OpenList) 把多個儲存服務的檔案列在同一個網頁裡，也能透過 WebDAV 提供。這個範本使用一體式映像檔，內含 FFmpeg 和 Aria2。

這個範本在你的網域後面跑一個 OpenList。管理員密碼和 Aria2 密碼都在部署時自動產生。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://openlist.example.com`。
2. 用 `admin` 登入。密碼在機密 `openlist-secrets` 的 `admin-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

   OpenList 每次重新啟動都會重設一次這個密碼，所以在網頁裡改的密碼會失效。要改密碼，請改機密再重新啟動（見「設定」）。
3. 要用離線下載時，Aria2 權杖在同一個機密的 `aria2-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret openlist-secrets -o jsonpath='{.data.aria2-password}' | base64 -d
   ```

   在 Aria2 設定裡，端點填 `http://127.0.0.1:6800/jsonrpc`，權杖填這個值。

## 設定

### 更改管理員密碼

把新的值寫進 `openlist-secrets` 的 `admin-password` 欄位，再重新啟動 StatefulSet，讓它讀到新值：

```sh
kubectl -n <namespace> rollout restart statefulset/openlist
```

### 使用離線下載

開啟管理頁面，進入設定，再進入其他，設好暫存資料夾。接著填入「開始使用」取得的 Aria2 端點和權杖。詳見 [Offline download](https://doc.oplist.org/guide/advanced/offline-download)。

### 用密碼保護資料夾

為路徑新增一筆 meta 記錄，並設定密碼。密碼要涵蓋子資料夾時，勾選密碼後面的 Apply to sub folder。透過 WebDAV 存取時，這個密碼不會生效。詳見 [Meta](https://doc.oplist.org/guide/advanced/meta)。
