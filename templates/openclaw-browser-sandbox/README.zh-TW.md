## 簡介

這個範本跑一個無頭 Chrome，讓 [OpenClaw](https://docs.openclaw.ai/tools/browser/remote) 或其他服務透過 Chrome DevTools 協定（CDP）操作。

這個瀏覽器只給單一可信任的擁有者使用。它沒有網頁，不用登入，也沒有權杖。只有同一個專案裡帶有信任呼叫端標籤的 Pod 連得進來。

## 開始使用

1. 在 Zeabur 打開 `openclaw-sandbox-browser` 服務。**概覽**顯示副本已就緒，服務就啟動好了。它沒有網域。
2. 確認你的用戶端 Pod 帶有標籤 `app.kubernetes.io/name`，值要和你在安裝表單填的信任呼叫端相同。搭配 OpenClaw 時，這個值是 `openclaw`。
3. 在該 Pod 裡，把下面的位址當成 CDP 端點：

   ```text
   http://openclaw-sandbox-browser:9222
   ```

   沒有這個標籤的 Pod 會被擋下來。這裡沒有自動產生的機密變數，所以能連進來的對象都能完全控制瀏覽器。

## 設定

### 連接 OpenClaw

把 `browser.profiles.<name>.cdpUrl` 設成上面的位址，並讓該設定檔成為預設：

```json5
{
  browser: {
    enabled: true,
    defaultProfile: "sandbox",
    profiles: {
      sandbox: { cdpUrl: "http://openclaw-sandbox-browser:9222" },
    },
  },
}
```

位址用 `http://` 開頭時，OpenClaw 會透過 `/json/version` 找到 WebSocket 端點。詳見 [Remote and hosted browsers](https://docs.openclaw.ai/tools/browser/remote)。

## 疑難排解

### 用戶端連不上

網路政策只放行帶有信任呼叫端標籤的 Pod，而且只開 9222 連接埠。確認用戶端在同一個專案裡，且它的 `app.kubernetes.io/name` 標籤和你在安裝表單填的值相同。詳見 [Remote and hosted browsers](https://docs.openclaw.ai/tools/browser/remote)。
