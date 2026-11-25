## Introduction

This template runs a PostgreSQL 18 database managed by the CloudNativePG operator. The extension [pgvector](https://github.com/pgvector/pgvector) is already enabled, so you can store vectors and search for the nearest ones with SQL.

The database is called `zeabur` and is owned by the user `app`. There is one instance with a 10Gi volume.

## Getting started

1. In Zeabur OSS, open the `postgresql` database in your project. When **Overview** shows it as ready, it is up. The database has no domain and no web page.
2. Apps in the same project connect to Service `postgresql-rw` on port `5432`, with database `zeabur` and user `app`. Under **Overview**, the **Connections** section shows the **Host**, **Port**, **Database** and **Username**. Click **Show** next to the password to read it. To give an app these values as variables, click **Connect to a service** in the same section.
3. The operator also creates Secret `postgresql-app`. No service in the project reads it as a variable, so read it with kubectl. The password is in key `password`:

   ```sh
   kubectl -n <namespace> get secret postgresql-app -o jsonpath='{.data.password}' | base64 -d
   ```

   Key `uri` holds a ready connection string:

   ```sh
   kubectl -n <namespace> get secret postgresql-app -o jsonpath='{.data.uri}' | base64 -d
   ```

   There is no superuser login. Use the connection string from an app or a client pod inside the project.

## Configuration

### Create a vector column and search it

Create a table with a vector column, insert rows, and order by a distance operator:

```sql
CREATE TABLE items (id bigserial PRIMARY KEY, embedding vector(3));
INSERT INTO items (embedding) VALUES ('[1,2,3]'), ('[4,5,6]');
SELECT * FROM items ORDER BY embedding <-> '[3,1,2]' LIMIT 5;
```

`<->` is L2 distance. pgvector also has `<#>` for inner product and `<=>` for cosine distance. See [Getting started](https://github.com/pgvector/pgvector#getting-started) in the pgvector docs.

### Add an index

Without an index, pgvector does an exact search. For large tables, add an HNSW index that matches the distance you use:

```sql
CREATE INDEX ON items USING hnsw (embedding vector_l2_ops);
```

Use `vector_cosine_ops` for `<=>` and `vector_ip_ops` for `<#>`. See [HNSW](https://github.com/pgvector/pgvector#hnsw).

### Back up the data

In Zeabur OSS, open the `postgresql` database and go to **Backups** → **Set up backups** to back up on a schedule to an S3 bucket. PostgreSQL backups need the Barman Cloud plugin. To get a copy now, click **More** at the top right of the database page, then **Export**. The browser downloads a `.dump` file made by `pg_dump --format=custom`. See [Set up S3 backups](https://oss.zeabur.com/docs/zeabur/deploy/databases/#set-up-s3-backups) and [Download a database dump](https://oss.zeabur.com/docs/zeabur/deploy/databases/#download-a-database-dump).

## Troubleshooting

### A query does not use the index

The query needs `ORDER BY` with a distance operator in ascending order, and a `LIMIT`. An expression such as `1 - (embedding <=> '[3,1,2]')` stops the planner from using the index. On a small table a table scan can be faster. See [Troubleshooting](https://github.com/pgvector/pgvector#troubleshooting).
