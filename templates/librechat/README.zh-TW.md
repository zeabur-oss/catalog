## 簡介

[LibreChat](https://github.com/danny-avila/LibreChat) 是 AI 模型的聊天應用程式。大家登入後和模型對話，也能搜尋自己的對話。

這個範本在你的網域後面跑 LibreChat，用 MongoDB 存資料，Meilisearch 做搜尋，並以含 pgvector 的 PostgreSQL 搜尋文件。聊天和文件嵌入都使用你在安裝表單選的 OpenAI API 金鑰。

## 開始使用

1. 等所有服務顯示為執行中，再開啟 `librechat` 服務的網域，例如 `https://chat.example.com`。
2. 範本在部署時用你在安裝表單填的管理員 Email 建立管理員帳號。取得它的密碼。密碼在機密 `app-secrets` 的 `admin-password` 欄位：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.admin-password}' | base64 -d
   ```

3. 用管理員 Email 和這組密碼登入。公開註冊已關閉，其他人無法自行註冊。之後再改機密，不會重設已有帳號的密碼。

`app-secrets` 裡其他的欄位（`mongo-password`、`jwt-secret`、`jwt-refresh-secret`、`creds-key`、`creds-iv` 和 `meili-key`）是 LibreChat 自己的服務使用。請在資料存在的期間一直保留。

## 設定

### 更換 OpenAI API 金鑰

金鑰在你於安裝表單選的機密的 `OPENAI_API_KEY` 欄位。把新的值寫進去，再重新啟動用到它的兩個部署：

```sh
kubectl -n <namespace> rollout restart deployment/librechat deployment/rag-api
```

詳見 [OpenAI](https://www.librechat.ai/docs/configuration/pre_configured_ai/openai)。
