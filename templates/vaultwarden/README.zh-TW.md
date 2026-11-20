## 簡介

[Vaultwarden](https://github.com/dani-garcia/vaultwarden) 是密碼管理器，可以搭配 Bitwarden 的應用程式和瀏覽器擴充功能使用。

這個範本在你的網域後面跑一個 Vaultwarden 伺服器。註冊功能是關閉的，由管理員在管理頁面逐一邀請使用者。管理頁面需要權杖，權杖在部署時自動產生。

## 開始使用

1. 在 Zeabur 打開 `vaultwarden` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，也就是你在安裝表單填的網域。
2. 取得管理員權杖。打開**環境變數**，在**來自設定檔和機密變數**點 `vaultwarden-secrets`，再按 `admin-token` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `vaultwarden-secrets` 的 `admin-token`：

   ```sh
   kubectl -n <namespace> get secret vaultwarden-secrets -o jsonpath='{.data.admin-token}' | base64 -d
   ```

3. 開啟 `https://<你的網域>/admin`，輸入權杖。
4. 在這個頁面用 Email 逐一邀請使用者。
5. 受邀的人開啟 `https://<你的網域>`，用受邀的 Email 註冊。之後在 Bitwarden 的應用程式裡，把你的網域當成伺服器位址登入。

## 設定

### 更換管理員權杖

在服務的**環境變數**分頁點 `vaultwarden-secrets`，填入新的 `admin-token`，檢查後套用。執行中的副本會繼續用舊值，直到你按服務頁上方的**重新啟動**。這個權杖就是管理頁面的密碼。詳見 [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page)。

## 疑難排解

### 管理頁面又要求輸入權杖

管理員工作階段預設只有 20 分鐘，請重新輸入權杖。詳見 [Enabling admin page](https://github.com/dani-garcia/vaultwarden/wiki/Enabling-admin-page)。

### 受邀的人沒有收到信

這個範本沒有設定 SMTP 伺服器，所以 Vaultwarden 不會寄信。請自己通知每位使用者，他們可以用受邀的 Email 註冊。
