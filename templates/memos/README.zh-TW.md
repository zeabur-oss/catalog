## 簡介

[Memos](https://usememos.com/) 是記短筆記的筆記服務，有標籤、搜尋和 API。你的筆記存在自己的伺服器上。

這個範本在你的網域後面跑一個 Memos。資料用 SQLite 存在 10 GiB 的磁碟區。公開註冊已關閉，第一次啟動時會建好名為 `admin` 的管理員帳號。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://memos.example.com`。
2. 以 `admin` 登入。密碼在機密 `memos-secrets` 的 `admin-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret memos-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 登入後在 Memos 裡改密碼。Memos 只在建立帳號時讀取這個機密，之後改機密不會改變密碼。
4. 註冊功能已關閉，其他帳號由 `admin` 在 Memos 的設定裡建立。

## 設定

### 建立個人存取權杖

在 Memos 的使用者設定裡建立權杖，再當作 Bearer 權杖送出。權杖以 `memos_pat_` 開頭，Memos 只會顯示一次。每個應用程式各用一個權杖，要撤銷時才不會影響其他應用程式。

```sh
curl -H "Authorization: Bearer memos_pat_..." https://memos.example.com/api/memos
```

詳見 Memos 文件的 [API Access](https://usememos.com/docs/integrations/api-access)。

## 疑難排解

### 設定是唯讀的，或無法儲存

設定檔宣告的每一組設定，Memos 都會在介面和 API 裡設為唯讀。範本在 ConfigMap `memos-config` 裡宣告了一般設定（General），其中包含註冊規則和自訂的個人檔案。Memos 把檔案當成整組設定，沒有寫的欄位會回到預設值。要修改這些設定，請編輯 ConfigMap，再重新啟動 StatefulSet：

```sh
kubectl -n <namespace> edit configmap memos-config
kubectl -n <namespace> rollout restart statefulset/memos
```

詳見 Memos 文件的 [Declarative configuration](https://usememos.com/docs/configuration/declarative-configuration)。

### 登入失敗

Memos 的使用者名稱區分大小寫，請輸入小寫的 `admin`。詳見 Memos 文件的 [Troubleshooting](https://usememos.com/docs/troubleshooting)。
