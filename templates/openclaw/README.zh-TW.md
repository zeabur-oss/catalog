## 簡介

[OpenClaw](https://github.com/openclaw/openclaw) 是個人 AI 助理。Gateway（閘道）負責執行助理，網頁控制介面（Control UI）讓你和它對話並管理它。它也能串接 WhatsApp、Telegram、Slack、Discord 等通訊軟體。

這個範本在你的網域後面跑一個 Gateway，使用你在安裝表單填的、相容 OpenAI 的模型服務商。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://openclaw.example.com`。
2. 登入畫面有一個 Gateway secret 欄位，把閘道權杖貼進去。權杖在機密 `openclaw-secrets` 的 `gateway-token` 欄位：

   ```sh
   kubectl -n <namespace> get secret openclaw-secrets -o jsonpath='{.data.gateway-token}' | base64 -d
   ```

3. 新的瀏覽器在你放行一次之前，會顯示 `disconnected (1008): pairing required`。請保持頁面開啟，列出請求，再放行你的那一筆。放行後頁面會自己連線：

   ```sh
   kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices list
   kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices approve <requestId>
   ```

4. 模型服務商就是你在安裝表單填的 base URL、模型和 API 金鑰。API 金鑰在你於表單裡指定的機密，欄位是 `api-key`，不在 `openclaw-secrets` 裡。

## 設定

### 更換服務商的 API 金鑰

把新的金鑰寫進你在安裝表單裡指定的機密的 `api-key` 欄位，再重新啟動 Deployment，讓它讀到新值：

```sh
kubectl -n <namespace> rollout restart deployment/openclaw
```

base URL 和模型只在第一次啟動時寫進 Gateway 的設定檔。之後再改安裝表單，不會改變它們。

### 移除已配對的瀏覽器

列出已配對的裝置，再移除不用的那一個：

```sh
kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices list
kubectl -n <namespace> exec deployment/openclaw -- node /app/openclaw.mjs devices remove <deviceId>
```

詳見 OpenClaw 文件的 [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair)。

## 疑難排解

### 另一個瀏覽器要求配對

每個瀏覽器設定檔各自配對，一個設定檔不能沿用另一個的放行。請對每個設定檔用 `devices approve` 放行一次，並用 `devices remove` 移除舊的項目。詳見 OpenClaw 文件的 [Connect and pair](https://docs.openclaw.ai/web/control-ui/connect-and-pair)。
