## 簡介

[Ollama](https://github.com/ollama/ollama) 執行大型語言模型，並透過 HTTP API 提供服務。

這個範本用 CPU 跑一個 Ollama，並準備了 50 GiB 的磁碟區放模型。它沒有網域、沒有網頁，也不用登入。只有同一個命名空間裡你允許的 Pod 連得到它的 API。不會預先載入任何模型。

## 開始使用

1. 在 Zeabur 開源版打開 `ollama` 服務。**概覽**顯示副本已就緒，服務就啟動好了。它沒有網域。
2. 其他工作負載用 Service `ollama` 的 `11434` 連接埠連到 Ollama，例如 `http://ollama:11434`。範本只允許同一個命名空間裡、帶有下面這個標籤的 Pod：

   ```yaml
   templates.zeabur.com/ollama-client: "true"
   ```

3. 要從你的電腦使用 API，先轉發連接埠，再用 `http://localhost:11434`。Zeabur 開源版沒有轉發連接埠的介面，請用 kubectl：

   ```sh
   kubectl -n <namespace> port-forward service/ollama 11434
   ```

4. 下載模型。請先看過模型的授權和大小。Pod 的記憶體上限是 8 GiB，所以放不下大型模型。按 `ollama` 服務頁上方的**終端機**，執行：

   ```sh
   ollama pull <模型>
   ```

這個服務沒有密碼，所以範本不會建立機密。

## 設定

### 連接 OpenAI 用戶端

Ollama 有相容 OpenAI 的 API。把用戶端的 base URL 設成 `http://ollama:11434/v1`。用戶端需要一個 API 金鑰的值，但 Ollama 會忽略它，所以填任何文字都可以。詳見 Ollama 文件的 [OpenAI compatibility](https://docs.ollama.com/api/openai-compatibility)。

### 列出和移除模型

按 `ollama` 服務頁上方的**終端機**，執行：

```sh
ollama ls
ollama rm <模型>
```

詳見 Ollama 文件的 [CLI reference](https://docs.ollama.com/cli)。

## 疑難排解

### Pod 連線逾時

Pod 需要「開始使用」裡的兩個標籤。請檢查連不上的 Pod 的標籤，並在該工作負載的 Pod 範本補上缺少的標籤。

### 模型很慢或載入不了

這個範本用 CPU 執行，記憶體上限是 8 GiB，一次只載入一個模型。按 `ollama` 服務頁上方的**終端機**，執行 `ollama ps` 看載入了什麼。模型放不下時，改選較小的模型。服務的**日誌**分頁會顯示原因。

```sh
ollama ps
```

詳見 Ollama 文件的 [FAQ](https://docs.ollama.com/faq) 和 [Troubleshooting](https://docs.ollama.com/troubleshooting)。
