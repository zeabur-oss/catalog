## 簡介

這個範本跑一個由 CloudNativePG 運算子管理的 PostgreSQL 18 資料庫。擴充套件 [pgvector](https://github.com/pgvector/pgvector) 已經啟用，你可以用 SQL 儲存向量，並搜尋最近的向量。

資料庫名稱是 `zeabur`，擁有者是使用者 `app`。只有一個執行個體，磁碟區為 10Gi。

## 開始使用

1. 等叢集 `postgresql` 顯示為就緒。資料庫沒有網域，也沒有網頁。
2. 同一個專案裡的應用程式連到 Service `postgresql-rw` 的 `5432` 連接埠，資料庫用 `zeabur`，使用者用 `app`。
3. 運算子會建立機密 `postgresql-app`。密碼在 `password` 欄位：

   ```sh
   kubectl -n <namespace> get secret postgresql-app -o jsonpath='{.data.password}' | base64 -d
   ```

   `uri` 欄位是現成的連線字串：

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

## 疑難排解

### 查詢沒有用到索引

查詢要用 `ORDER BY` 搭配遞增的距離運算子，並加上 `LIMIT`。像 `1 - (embedding <=> '[3,1,2]')` 這樣的運算式會讓規劃器不使用索引。資料表很小時，全表掃描可能更快。詳見 [Troubleshooting](https://github.com/pgvector/pgvector#troubleshooting)。
