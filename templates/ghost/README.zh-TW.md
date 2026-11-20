## 簡介

[Ghost](https://github.com/TryGhost/Ghost) 是用來經營部落格和付費電子報的發布平台。這個範本在你的網域後面跑 Ghost 6，MySQL 放在磁碟區上。

Ghost 透過你在安裝表單填寫的 SMTP 伺服器寄出登入信和系統信件。

## 開始使用

1. 在 Zeabur 打開 `ghost` 服務。**概覽**顯示副本已就緒，服務就啟動好了。網域在**網路**分頁的**公開網域**，例如 `blog.example.com`。管理後台在這個網域的 `/ghost`。
2. 設定工作會用你在安裝表單填的擁有者 Email 建立網站擁有者，所以不用再填設定表單。取得擁有者密碼。在 `ghost` 服務打開**環境變數**，在**來自設定檔和機密變數**點 `ghost-secrets`，再按 `admin-password` 旁的眼睛圖示。不用 Zeabur 時，用 kubectl 讀取機密變數 `ghost-secrets` 的 `admin-password`：

   ```sh
   kubectl -n <namespace> get secret ghost-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 開啟管理後台，用擁有者 Email 和這組密碼登入。

`ghost-secrets` 裡其他的欄位（`database-password` 和 `root-password`）是給 MySQL 用的。Ghost 不會用到 `root-password`。

## 設定

### 寄送電子報

SMTP 只負責系統信件。要把文章寄成電子報，Ghost 需要大量寄信的服務，目前只支援 Mailgun。在管理後台開啟電子報的電子郵件設定，輸入你的 Mailgun API 金鑰。詳見 [Newsletters](https://docs.ghost.org/newsletters) 和 [Why do I have to set up Mailgun?](https://docs.ghost.org/faq/mailgun-newsletters)。

### 備份資料

Ghost 把文章、會員和設定存在 MySQL，放在磁碟區 `ghost-mysql-data`。圖片、佈景主題和其他上傳的檔案放在磁碟區 `ghost-data`，掛載在 `/var/lib/ghost/content`。`ghost-mysql` 是單純的 MySQL 容器，不是 Zeabur 資料庫，所以沒有**備份**分頁。用 kubectl 把它傾印成你電腦上的檔案：

```sh
kubectl -n <namespace> exec statefulset/ghost-mysql -- sh -c 'MYSQL_PWD="$MYSQL_PASSWORD" mysqldump --single-transaction --no-tablespaces -h127.0.0.1 -ughost ghost' > ghost.sql
```

內容資料夾用 `ghost` 服務的**檔案**分頁複製。資料夾很大時，改用 kubectl：

```sh
kubectl -n <namespace> cp ghost-0:/var/lib/ghost/content ./ghost-content
```

Ghost 管理後台的「設定 > 進階 > 匯入/匯出」可以把文章和設定匯出成 JSON，但自架網站的圖片不在裡面。詳見 Ghost 文件的 [Exporting content and data](https://ghost.org/help/exports/)。

### 更換資料庫密碼

Ghost 從機密變數讀取 `database-password`，但 MySQL 只在資料目錄是空的時候才設定 `ghost` 使用者的密碼。只改機密變數的話，Ghost 會連不上資料庫。要先在 MySQL 裡改。在 `ghost-mysql` 服務打開**終端機**，執行：

```sh
mysql --socket=/var/run/mysqld/mysqld.sock -uroot -p"$MYSQL_ROOT_PASSWORD" -e "ALTER USER 'ghost'@'%' IDENTIFIED BY '<new password>'"
```

接著在 `ghost-secrets` 為 `database-password` 填入同一個值，檢查後套用，再按 `ghost` 和 `ghost-mysql` 服務頁上方的**重新啟動**。詳見 [MySQL 映像檔](https://hub.docker.com/_/mysql)文件的環境變數說明。

## 疑難排解

### 登入信或系統信件寄不出去，出現連線逾時

有些主機會封鎖對外的 SMTP 連接埠。請改用你的郵件服務和網路都允許的 SMTP 連接埠。詳見 [Configuration](https://docs.ghost.org/config)。

### 頁面顯示 502 或打不開

在 Zeabur 打開 `ghost` 服務。確認**概覽**顯示副本已就緒，再看**日誌**分頁。副本沒有就緒時，打開**部署紀錄**，切到**事件**。事件出現 `OOMKilled`，表示服務的記憶體不夠，到**設定** → **運算資源**調高**記憶體上限**。服務變慢或一直重新連線時，打開**用量**分頁，出現**接近記憶體上限**就用同樣的方式調高上限。網域或憑證一直沒有完成時，見網域文件的[遇到問題](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/#遇到問題)。

不用 Zeabur 時，用 `kubectl -n <namespace> logs statefulset/ghost` 看日誌，用 `kubectl -n <namespace> get events --sort-by=.lastTimestamp` 看事件。

詳見 Zeabur 文件的[調整副本、資源與健康檢查](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/service-settings/)和[查看日誌、事件與用量，開啟終端機](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/logs-terminal/)。
