## 簡介

[SearXNG](https://github.com/searxng/searxng) 是整合搜尋引擎。它把你的查詢送到許多搜尋引擎，再把結果合併成一份清單，不需要帳號，SearXNG 本身也不追蹤你。

這個範本在你的網域後面跑一個 SearXNG。HTML 和 JSON 兩種結果都已開啟。

## 開始使用

1. 等服務顯示為執行中，開啟它的網域，例如 `https://searxng.example.com`。
2. 直接搜尋。這裡沒有帳號，也不用登入，知道網址的人都能使用。
3. 伺服器的簽章金鑰在機密 `app-secrets` 的 `signing-key` 欄位。搜尋時用不到：

   ```sh
   kubectl -n <namespace> get secret app-secrets -o jsonpath='{.data.signing-key}' | base64 -d
   ```

## 設定

### 取得 JSON 結果

在搜尋請求加上 `format=json`：

```sh
curl 'https://<你的網域>/search?q=searxng&format=json'
```

詳見 [Search API](https://docs.searxng.org/dev/search_api.html)。

## 疑難排解

### 使用 format=csv 或 format=rss 的請求得到 403

這個範本只開啟 `html` 和 `json` 兩種格式。沒開啟的格式會得到 `403 Forbidden`。詳見 [Search API](https://docs.searxng.org/dev/search_api.html)。
