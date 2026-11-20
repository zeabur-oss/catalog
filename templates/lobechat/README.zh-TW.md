## 簡介

[LobeChat](https://github.com/lobehub/lobe-chat) 是 AI 模型的聊天介面。每個人的對話都存在自己的瀏覽器裡。

這個範本在你的網域後面跑 LobeChat。頁面由存取碼保護，伺服器使用你在安裝表單選的 OpenAI API 金鑰。沒有使用者帳號。

## 開始使用

1. 等服務顯示為執行中，再開啟它的網域，例如 `https://chat.example.com`。
2. LobeChat 要求輸入存取碼時，填入存取碼。存取碼在機密 `lobechat-secrets` 的 `access-code` 欄位：

   ```sh
   kubectl -n <namespace> get secret lobechat-secrets -o jsonpath='{.data.access-code}' | base64 -d
   ```

3. 開始聊天。對話使用你在安裝表單選的機密裡的 OpenAI 金鑰（`api-key` 欄位）。每個人的對話紀錄都留在自己的瀏覽器裡。

## 設定

### 更換存取碼

把新的值寫進 `lobechat-secrets` 的 `access-code` 欄位，再重新啟動部署，讓它讀到新值：

```sh
kubectl -n <namespace> rollout restart deployment/lobechat
```

請把新的存取碼告訴所有使用這個頁面的人。詳見 [ACCESS_CODE](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/basic.mdx)。

### 更換 OpenAI API 金鑰

把新的金鑰寫進你在安裝表單選的機密的 `api-key` 欄位，再重新啟動部署：

```sh
kubectl -n <namespace> rollout restart deployment/lobechat
```

詳見 [OPENAI_API_KEY](https://github.com/lobehub/lobe-chat/blob/v1.143.3/docs/self-hosting/environment-variables/model-provider.mdx)。
