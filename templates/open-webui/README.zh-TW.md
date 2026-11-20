## 簡介

[Open WebUI](https://github.com/open-webui/open-webui) 是大型語言模型的聊天介面。你把它連上模型服務商，大家就能在瀏覽器裡和模型對話。

這個範本在你的網域後面跑 Open WebUI，並附帶供 pipeline 使用的 `pipeline` 服務。註冊功能已關閉，也還沒有連接任何模型服務商。

## 開始使用

1. 在 Zeabur 打開 `open-webui` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，就是你在安裝表單填的網域，例如 `chat.example.com`。
2. 用你在安裝表單填的管理員 Email 登入。密碼從 `open-webui` 服務取得：打開**環境變數**，在**來自設定檔和機密變數**點 `app-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `app-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. Open WebUI 在第一次啟動、還沒有任何使用者時建立管理員帳號。之後改機密不會改變密碼，要改請在 Open WebUI 裡改。
4. 註冊功能已關閉，其他使用者由管理員建立。
5. 新增模型服務商。請見「設定」。
6. 機密 `app-secrets` 裡還有兩個日常使用不需要的欄位。`session-key` 是網頁介面的密鑰，`pipeline-key` 是網頁介面呼叫 `pipeline` 服務用的 API 金鑰。取得方式同上。不用 Zeabur 時，用 kubectl：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.session-key}' | base64 -d
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.pipeline-key}' | base64 -d
   ```

## 設定

### 新增模型服務商

以管理員登入，開啟 Settings，再到 Connections（管理員區塊）。在 Manage OpenAI API Connections 旁選 Add Connection。填入服務商相容 OpenAI 的 API 網址和 API 金鑰，再儲存。OpenAI 的網址是 `https://api.openai.com/v1`。詳見 Open WebUI 文件的 [OpenAI](https://docs.openwebui.com/getting-started/quick-start/connect-a-provider/starting-with-openai)。

範本啟動 Open WebUI 時，把 `ENABLE_PERSISTENT_CONFIG` 設為 `false`。這對你改的設定有什麼影響，請見「疑難排解」。

## 疑難排解

### 在管理員設定裡改的設定，重新啟動後不見了

`ENABLE_PERSISTENT_CONFIG` 設為 `false` 時，Open WebUI 每次啟動都讀取環境變數裡的設定。你可以在管理員設定裡編輯，但改動不會被儲存。重新啟動後請再改一次。詳見 Open WebUI 文件的 [Environment variable configuration](https://docs.openwebui.com/reference/env-configuration)。
