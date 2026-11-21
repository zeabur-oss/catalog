## 简介

这个模板运行一个由 CloudNativePG 运算符管理的 PostgreSQL 18 数据库。扩展 [pgvector](https://github.com/pgvector/pgvector) 已经启用，你可以用 SQL 存储向量，并搜索最近的向量。

数据库名称是 `zeabur`，所有者是用户 `app`。只有一个实例，卷为 10Gi。

## 开始使用

1. 在 Zeabur 开源版的项目里打开 `postgresql` 数据库。**概览**显示它已就绪，就启动好了。数据库没有域名，也没有网页。
2. 同一个项目里的应用连到 Service `postgresql-rw` 的 `5432` 端口，数据库用 `zeabur`，用户用 `app`。在**概览**的**连接**区块可以看到**主机**、**端口**、**数据库**和**用户名**，点密码旁的**显示**就能读取密码。要把这些值当成变量交给应用，点同一区块的**连接到服务**。
3. 运算符还会创建机密变量 `postgresql-app`。项目里没有服务把它当成变量读取，所以用 kubectl 读取。密码在 `password`：

   ```sh
   kubectl -n <namespace> get secret postgresql-app -o jsonpath='{.data.password}' | base64 -d
   ```

   `uri` 是现成的连接字符串：

   ```sh
   kubectl -n <namespace> get secret postgresql-app -o jsonpath='{.data.uri}' | base64 -d
   ```

   这里不能用超级用户登录。请在项目内的应用或客户端 Pod 里使用这个连接字符串。

## 配置

### 创建向量列并搜索

创建带向量列的表，写入数据，再用距离运算符排序：

```sql
CREATE TABLE items (id bigserial PRIMARY KEY, embedding vector(3));
INSERT INTO items (embedding) VALUES ('[1,2,3]'), ('[4,5,6]');
SELECT * FROM items ORDER BY embedding <-> '[3,1,2]' LIMIT 5;
```

`<->` 是 L2 距离。pgvector 也有内积的 `<#>` 和余弦距离的 `<=>`。详见 pgvector 文档的 [Getting started](https://github.com/pgvector/pgvector#getting-started)。

### 添加索引

没有索引时，pgvector 做的是精确搜索。表很大时，添加和你所用距离相符的 HNSW 索引：

```sql
CREATE INDEX ON items USING hnsw (embedding vector_l2_ops);
```

`<=>` 用 `vector_cosine_ops`，`<#>` 用 `vector_ip_ops`。详见 [HNSW](https://github.com/pgvector/pgvector#hnsw)。

### 备份数据

在 Zeabur 开源版打开 `postgresql` 数据库，到**备份** → **设置备份**，就能按计划备份到 S3 存储桶。PostgreSQL 备份需要 Barman Cloud 插件。要现在就拿一份数据，在数据库页面右上角点击**更多** → **导出**，浏览器会下载 `pg_dump --format=custom` 生成的 `.dump` 文件。详见[配置 S3 备份](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#配置-s3-备份)和[下载数据库转储文件](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/databases/#下载数据库转储文件)。

## 故障排除

### 查询没有用到索引

查询要用 `ORDER BY` 搭配升序的距离运算符，并加上 `LIMIT`。像 `1 - (embedding <=> '[3,1,2]')` 这样的表达式会让规划器不使用索引。表很小时，全表扫描可能更快。详见 [Troubleshooting](https://github.com/pgvector/pgvector#troubleshooting)。
