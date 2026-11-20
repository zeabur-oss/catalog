## 簡介

[Miniflux](https://miniflux.app/) 是簡約的 RSS 閱讀器。它會抓取你訂閱的 RSS 和 Atom 來源，把新文章集中在一個地方。

這個範本在你的網域後面跑一個 Miniflux，並附帶專屬的 PostgreSQL 資料庫。第一次啟動時會建好名為 `admin` 的管理員帳號。

## 開始使用

1. 在 Zeabur 打開 `miniflux` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `miniflux.example.com`。
2. 以 `admin` 登入。密碼從 `miniflux` 服務取得：打開**環境變數**，在**來自設定檔和機密變數**點 `miniflux-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `miniflux-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret miniflux-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 新增第一個訂閱來源。
4. 需要的話可以在 Miniflux 裡改密碼。Miniflux 只在建立帳號時讀取這個機密，之後改機密不會改變密碼。

## 設定

### 建立 API 金鑰

前往 Settings，再到 API Keys，選 Create a new API key。把金鑰放在 `X-Auth-Token` 標頭送出：

```sh
curl -H "X-Auth-Token: <key>" https://miniflux.example.com/v1/me
```

詳見 Miniflux 文件的 [API](https://miniflux.app/docs/api.html)。

### 使用閱讀器 App

許多閱讀器 App 可以透過 Google Reader API 或 Fever API 連上 Miniflux。在 Miniflux 裡開啟 Settings，再到 Integrations，為 API 選一組使用者名稱和密碼。在 App 裡填入這組使用者名稱和密碼。支援的 App 請見 Miniflux 文件的 [Google Reader API](https://miniflux.app/docs/google_reader.html) 和 [Fever API](https://miniflux.app/docs/fever.html)。

## 疑難排解

### 私有位址的訂閱來源被拒絕

Miniflux 預設不會連到私有和本機迴路的網路位址，範本也沒有改動這項設定。請改用公開網際網路連得到的訂閱來源位址。詳見 [Configuration parameters](https://miniflux.app/docs/configuration.html) 裡的 `FETCHER_ALLOW_PRIVATE_NETWORKS`。

### 手動重新整理沒有立刻更新

Miniflux 預設每 30 分鐘才允許手動重新整理一次（`FORCE_REFRESH_INTERVAL`），其餘時間由它在背景依自己的排程更新訂閱來源。詳見 [Configuration parameters](https://miniflux.app/docs/configuration.html)。
