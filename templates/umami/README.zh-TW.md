## 簡介

[Umami](https://umami.is/) 是網站流量分析的儀表板。你在網頁上加一小段追蹤指令碼，就能在儀表板看到造訪數據。

這個範本在你的網域後面跑一個 Umami，並附帶它自己的 PostgreSQL 資料庫（Service `umami-db-rw`，連接埠 `5432`），由 CloudNativePG 運算子管理。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://umami.example.com`。
2. 用 `admin` 登入。密碼在機密 `umami-secrets` 的 `admin-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret umami-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

   範本在第一次啟動時，已經替換掉公開的預設密碼。登入後，請在 Umami 裡更改這個密碼。
3. 新增你的網站，再把追蹤指令碼放進網站的頁面（見「設定」）。

同一個機密還有 `app-secret` 欄位，用來簽署登入權杖和分享連結。使用 Umami 時用不到。

## 設定

### 新增網站並安裝追蹤程式碼

點側邊欄的 Websites，再點 Add website，輸入名稱和網域後儲存。點該網站的 Edit，複製 Tracking code 底下的程式碼，放進頁面的 `<head>` 區段。詳見 [Add a website](https://docs.umami.is/docs/add-a-website) 和 [Collect data](https://docs.umami.is/docs/collect-data)。

### 更改你的密碼

點側邊導覽列的個人檔案按鈕，依序進入 Settings、Security，再點 Change password。Umami 3.5.0 之前，這個按鈕在 Profile 頁面，不在 Security。詳見 [Login](https://docs.umami.is/docs/login)。

## 疑難排解

### 資料沒有出現

在你的網站上開啟瀏覽器開發者工具，在 Network 分頁找出送到你的 Umami 網域的請求，確認指令碼載入時沒有錯誤。廣告攔截器可能讓指令碼無法載入。詳見 [Collect data](https://docs.umami.is/docs/collect-data)。
