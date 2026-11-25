## 簡介

[SillyTavern](https://github.com/SillyTavern/SillyTavern) 是大型語言模型的聊天前端。它連到你選的模型 API，並保存你的角色和對話。

這個範本在你的網域後面跑一個 SillyTavern。存取受一組 Basic 驗證密碼保護，密碼在部署時自動產生。對話和設定存在磁碟區裡。

## 開始使用

1. 在 Zeabur 開源版打開 `sillytavern` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，也就是你在安裝表單填的網域。
2. 瀏覽器會要求登入。使用者名稱是 `user`。密碼在機密變數 `app-secrets` 的 `basic-password`。打開**環境變數**，在**來自設定檔和機密變數**點 `app-secrets`，再按 `basic-password` 旁的眼睛圖示。不用 Zeabur 開源版時，用 kubectl 讀取機密變數 `app-secrets` 的 `basic-password`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.basic-password}' | base64 -d
   ```

3. 沒有另外的使用者帳號。登入後，設定你的模型 API 連線（見「設定」）。

## 設定

### 連接模型 API

點上方列的 API Connections，選擇 API 類型和來源，輸入你的 API 金鑰，再點 Connect。詳見 [Quick Start](https://docs.sillytavern.app/usage/quick-start/) 和 [API Connections](https://docs.sillytavern.app/usage/api-connections/)。

### 更改密碼

在服務的**環境變數**分頁點 `app-secrets`，填入新的 `basic-password`，檢查後套用。執行中的副本會繼續用舊值，直到你按服務頁上方的**重新啟動**。下次開啟頁面時，瀏覽器會要求輸入新密碼。

## 疑難排解

### 頁面顯示 502 或打不開

在 Zeabur 開源版打開 `sillytavern` 服務，確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本一直重新啟動時，打開**部署紀錄**，切到**事件**。如果事件寫著 `OOMKilled`，到**設定** → **運算資源**調高**記憶體上限**。詳見[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。

網域或憑證一直停在等待中時，看[加上公開網域與 HTTPS](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)的「遇到問題」。

### 瀏覽器一直要求登入

使用者名稱用 `user`，密碼用 `app-secrets` 的 `basic-password` 欄位。更改密碼之後，瀏覽器儲存的舊密碼就不能用了。
