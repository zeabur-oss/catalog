## 簡介

[New API](https://github.com/QuantumNous/new-api) 是 AI 模型的閘道。它給你的應用程式一組相容 OpenAI 的 API，並把每個請求送到你設定好的模型服務商。

這個範本在你的網域後面跑一個 New API，並附帶專屬的 PostgreSQL 資料庫。初始設定已經替你完成，所以不會出現設定頁面，名為 `admin` 的管理員帳號也已建好。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://newapi.example.com`。
2. 以 `admin` 登入。密碼是 `Aa1-` 加上機密 `newapi-secrets` 的 `admin-password` 欄位的值。下面的指令會印出完整的密碼：

   ```sh
   echo "Aa1-$(kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.admin-password}' | base64 -d)"
   ```

3. 把你的模型服務商加成管道（channel）。詳見「設定」。
4. 機密 `newapi-secrets` 裡還有兩個日常使用不需要的欄位。`session-secret` 是工作階段機密，`crypto-secret` 是快取鍵用的 HMAC 機密：

   ```sh
   kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.session-secret}' | base64 -d
   kubectl -n <namespace> get secret newapi-secrets -o jsonpath='{.data.crypto-secret}' | base64 -d
   ```

## 設定

### 新增模型服務商

管道（channel）是對一個模型服務商的一條連線。以 `admin` 登入後，開啟左側選單的 Channels，選 Add Channel。選擇服務商類型，填入名稱和服務商的 API 金鑰，並勾選這個管道支援的模型。用管道上的 Test 確認能正常運作。詳見 New API 文件的 [Channel Management](https://docs.newapi.ai/en/docs/guide/feature-guide/admin/channel)。

### 從應用程式呼叫閘道

在主控台建立 API 權杖，詳見 [API Tokens](https://docs.newapi.ai/en/docs/guide/console/api-token)。接著把應用程式指向 `https://<你的網域>/v1`，並把權杖當作 Bearer 權杖送出：

```sh
curl https://newapi.example.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-<token>" \
  -d '{"model":"<model>","messages":[{"role":"user","content":"Hello"}]}'
```

詳見 New API 文件的 [Using API and Playground](https://docs.newapi.ai/en/docs/guide/feature-guide/user/api)。

## 疑難排解

### 用自動產生的密碼登入失敗

密碼不只是機密的值，它是以 `Aa1-` 開頭的。請用「開始使用」裡的指令印出完整的密碼。
