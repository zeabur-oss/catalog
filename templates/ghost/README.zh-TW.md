## 簡介

[Ghost](https://github.com/TryGhost/Ghost) 是用來經營部落格和付費電子報的發布平台。這個範本在你的網域後面跑 Ghost 6，MySQL 放在磁碟區上。

Ghost 透過你在安裝表單填寫的 SMTP 伺服器寄出登入信和系統信件。

## 開始使用

1. 等設定工作完成、服務顯示為執行中。網站在你選的網域，管理後台在 `/ghost`，例如 `https://blog.example.com/ghost`。
2. 設定工作會用你在安裝表單填的擁有者 Email 建立網站擁有者，所以不用再填設定表單。取得擁有者密碼。密碼在機密 `ghost-secrets` 的 `admin-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret ghost-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 開啟管理後台，用擁有者 Email 和這組密碼登入。

`ghost-secrets` 裡其他的欄位（`database-password` 和 `root-password`）是給 MySQL 用的。Ghost 不會用到 `root-password`。

## 設定

### 寄送電子報

SMTP 只負責系統信件。要把文章寄成電子報，Ghost 需要大量寄信的服務，目前只支援 Mailgun。在管理後台開啟電子報的電子郵件設定，輸入你的 Mailgun API 金鑰。詳見 [Newsletters](https://docs.ghost.org/newsletters) 和 [Why do I have to set up Mailgun?](https://docs.ghost.org/faq/mailgun-newsletters)。

## 疑難排解

### 登入信或系統信件寄不出去，出現連線逾時

有些主機會封鎖對外的 SMTP 連接埠。請改用你的郵件服務和網路都允許的 SMTP 連接埠。詳見 [Configuration](https://docs.ghost.org/config)。
