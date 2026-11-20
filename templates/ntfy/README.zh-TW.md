## 簡介

[ntfy](https://ntfy.sh/) 讓腳本和應用程式透過一般的 HTTP 請求，把推播通知送到你的手機或桌面。你把訊息發到某個主題，訂閱這個主題的所有用戶端都會收到。

這個範本在你的網域後面跑一個 ntfy 伺服器。每個主題都需要帳號，訊息和使用者存在 5 GiB 的磁碟區。每次啟動都會建好名為 `admin` 的使用者。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://ntfy.example.com`。
2. 以 `admin` 登入。密碼在機密 `ntfy-secrets` 的 `admin-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret ntfy-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 所有主題預設都拒絕存取，所以要發送或訂閱的應用程式，必須用這個帳號或存取權杖登入。請見「設定」。

## 設定

### 更換 admin 密碼

ntfy 每次啟動都會套用這個機密。要換密碼，把新的值寫進 `ntfy-secrets` 的 `admin-password` 欄位，再重新啟動 StatefulSet：

```sh
kubectl -n <namespace> patch secret ntfy-secrets --type merge -p '{"stringData":{"admin-password":"<新密碼>"}}'
kubectl -n <namespace> rollout restart statefulset/ntfy
```

### 新增使用者

在 Pod 裡執行 `ntfy user add`，再用 `ntfy access` 讓使用者能存取某個主題。`rw` 表示可以發送也可以訂閱：

```sh
kubectl -n <namespace> exec -it statefulset/ntfy -- ntfy user add <使用者名稱>
kubectl -n <namespace> exec statefulset/ntfy -- ntfy access <使用者名稱> <主題> rw
```

詳見 ntfy 文件的 [Access control](https://docs.ntfy.sh/config/)。

### 建立存取權杖

存取權杖讓應用程式不用你的密碼就能發送或訂閱。登入網頁版後，在 Account 區塊就能建立。也可以在 Pod 裡建立：

```sh
kubectl -n <namespace> exec statefulset/ntfy -- ntfy token add --label=<標籤> admin
```

把權杖當作 Bearer 權杖送出：

```sh
curl -H "Authorization: Bearer tk_..." -d "Hello" https://ntfy.example.com/mytopic
```

詳見 ntfy 文件的 [Publishing](https://docs.ntfy.sh/publish/)。

## 疑難排解

### 用戶端無法發送或訂閱

範本預設拒絕存取每個主題。請讓用戶端用使用者名稱和密碼，或存取權杖登入。`admin` 以外的使用者，還要用 `ntfy access` 讓它能存取該主題。詳見 ntfy 文件的 [Access control](https://docs.ntfy.sh/config/)。
