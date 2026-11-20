## 簡介

[LiteLLM](https://github.com/BerriAI/litellm) 是代理，在多家 LLM 服務前面提供一個相容 OpenAI 的 API。它也有管理頁面，用來管理模型、金鑰和花費。

這個範本在你的網域後面跑 LiteLLM，模型、金鑰和花費都存在 PostgreSQL。

## 開始使用

1. 等服務顯示為執行中，再開啟網域的 `/ui`，例如 `https://litellm.example.com/ui`。
2. 取得主金鑰。金鑰在機密 `litellm-secrets` 的 `master-key` 欄位：

   ```sh
   kubectl -n <namespace> get secret litellm-secrets -o jsonpath='{.data.master-key}' | base64 -d
   ```

   實際使用的金鑰是 `sk-` 加上這個值。
3. 用 `admin` 登入，密碼是 `sk-` 加上剛才讀到的值。
4. 同一把 `sk-` 金鑰也是呼叫 `https://litellm.example.com` 的管理員 API 金鑰。模型和給用戶端的金鑰都在管理頁面裡新增。

`litellm-secrets` 的 `salt-key` 用來加密資料庫裡模型服務的認證資訊。加入模型後，請不要再更改它。

## 設定

### 新增模型

在管理頁面開啟 Models + Endpoints，選擇模型服務並填入它的 API 金鑰。在頁面新增的模型存在資料庫裡，不用編輯檔案，也不用重新啟動。詳見 [Model Management](https://docs.litellm.ai/docs/proxy/model_management)。

### 為用戶端建立金鑰

請不要把主金鑰交給應用程式。在管理頁面為每個用戶端建立虛擬金鑰，再把用戶端指向 `https://litellm.example.com/v1` 並使用這把金鑰。詳見 [Virtual Keys](https://docs.litellm.ai/docs/proxy/virtual_keys) 和 [Langchain, OpenAI SDK 等用戶端](https://docs.litellm.ai/docs/proxy/user_keys)。
