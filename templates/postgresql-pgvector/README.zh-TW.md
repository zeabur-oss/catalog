## 簡介

這個範本跑一個由 CloudNativePG 運算子管理的 PostgreSQL 18 資料庫。擴充套件 [pgvector](https://github.com/pgvector/pgvector) 已經啟用，你可以用 SQL 儲存向量，並搜尋最近的向量。

資料庫名稱是 `zeabur`，擁有者是使用者 `app`。只有一個執行個體，磁碟區為 10Gi。

## 開始使用

1. 在 Zeabur 的專案裡打開 `postgresql` 資料庫。**概覽**顯示它已就緒，就啟動好了。資料庫沒有網域，也沒有網頁。
2. 同一個專案裡的應用程式連到 Service `postgresql-rw` 的 `5432` 連接埠，資料庫用 `zeabur`，使用者用 `app`。在**概覽**的**連線**區塊可以看到**主機**、**連接埠**、**資料庫**和**使用者名稱**，按密碼旁的**顯示**就能讀取密碼。要把這些值當成變數交給應用程式，按同一區塊的**接到服務**。
3. 運算子還會建立機密變數 `postgresql-app`。專案裡沒有服務把它當成變數讀取，所以用 kubectl 讀取。密碼在 `password`：

   ```sh
   kubectl -n <namespace> get secret postgresql-app -o jsonpath='{.data.password}' | base64 -d
   ```

   `uri` 是現成的連線字串：

   ```sh
   kubectl -n <namespace> get secret postgresql-app -o jsonpath='{.data.uri}' | base64 -d
   ```

   這裡不能用超級使用者登入。請在專案內的應用程式或用戶端 Pod 裡使用這個連線字串。

## 設定

### 建立向量欄位並搜尋

建立有向量欄位的資料表，寫入資料，再用距離運算子排序：

```sql
CREATE TABLE items (id bigserial PRIMARY KEY, embedding vector(3));
INSERT INTO items (embedding) VALUES ('[1,2,3]'), ('[4,5,6]');
SELECT * FROM items ORDER BY embedding <-> '[3,1,2]' LIMIT 5;
```

`<->` 是 L2 距離。pgvector 也有內積的 `<#>` 和餘弦距離的 `<=>`。詳見 pgvector 文件的 [Getting started](https://github.com/pgvector/pgvector#getting-started)。

### 加上索引

沒有索引時，pgvector 做的是精確搜尋。資料表很大時，加上和你所用距離相符的 HNSW 索引：

```sql
CREATE INDEX ON items USING hnsw (embedding vector_l2_ops);
```

`<=>` 用 `vector_cosine_ops`，`<#>` 用 `vector_ip_ops`。詳見 [HNSW](https://github.com/pgvector/pgvector#hnsw)。

### 備份資料

在 Zeabur 打開 `postgresql` 資料庫，到**備份** → **設定備份**，就能依排程備份到 S3 儲存貯體。PostgreSQL 備份需要 Barman Cloud 外掛。要現在就拿一份資料，在資料庫頁面右上角按**更多** → **匯出**，瀏覽器會下載 `pg_dump --format=custom` 產生的 `.dump` 檔。詳見[設定 S3 備份](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#設定-s3-備份)和[下載資料庫傾印檔](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/databases/#下載資料庫傾印檔)。

## 疑難排解

### 查詢沒有用到索引

查詢要用 `ORDER BY` 搭配遞增的距離運算子，並加上 `LIMIT`。像 `1 - (embedding <=> '[3,1,2]')` 這樣的運算式會讓規劃器不使用索引。資料表很小時，全表掃描可能更快。詳見 [Troubleshooting](https://github.com/pgvector/pgvector#troubleshooting)。
