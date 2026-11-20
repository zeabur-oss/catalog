## 簡介

[SillyTavern](https://github.com/SillyTavern/SillyTavern) 是大型語言模型的聊天前端。它連到你選的模型 API，並保存你的角色和對話。

這個範本在你的網域後面跑一個 SillyTavern。存取受一組 Basic 驗證密碼保護，密碼在部署時自動產生。對話和設定存在磁碟區裡。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://sillytavern.example.com`。
2. 瀏覽器會要求登入。使用者名稱是 `user`。密碼在機密 `app-secrets` 的 `basic-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.basic-password}' | base64 -d
   ```

3. 沒有另外的使用者帳號。登入後，設定你的模型 API 連線（見「設定」）。

## 設定

### 連接模型 API

點上方列的 API Connections，選擇 API 類型和來源，輸入你的 API 金鑰，再點 Connect。詳見 [Quick Start](https://docs.sillytavern.app/usage/quick-start/) 和 [API Connections](https://docs.sillytavern.app/usage/api-connections/)。

### 更改密碼

把新的值寫進 `app-secrets` 的 `basic-password` 欄位，再重新啟動部署，讓它讀到新值：

```sh
kubectl -n <namespace> rollout restart deployment/sillytavern
```

下次開啟頁面時，瀏覽器會要求輸入新密碼。

## 疑難排解

### 瀏覽器一直要求登入

使用者名稱用 `user`，密碼用 `app-secrets` 的 `basic-password` 欄位。更改密碼之後，瀏覽器儲存的舊密碼就不能用了。
