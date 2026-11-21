## 簡介

Falco 監看每個節點上的系統呼叫，找出容器內的可疑行為，例如開啟 shell 或提升權限。它用內建的 eBPF 探針，不需要核心模組。規則只在 chart 升級時更新。

想在 Zeabur 開源版看到執行期警示時，就裝它。Falcosidekick 會把每則警示送到相關 Namespace 的 `PolicyReport`；沒有 Namespace 的警示，則送到 `ClusterPolicyReport`。每份報告只保留最新的 100 則。警示帶有指令列和檔案路徑，所以只有 Namespace 管理員讀得到自己 Namespace 的報告。要讓其他人也能讀，在該 Namespace 綁定 ClusterRole `falco-alert-reader`。

Falco 在每個節點以特權模式執行。Falcosidekick 讀取 Pod 和節點的中繼資料，並在每個 Namespace 寫入報告。裝了 VictoriaMetrics 時，優先等級為 warning 以上的警示也會送到叢集的通知管道，不會送到專案的管道。

## 安裝前準備

每個節點的核心要有 BTF，也就是 Linux 5.8 以上。確認每個節點都有 `/sys/kernel/btf/vmlinux`。

## 使用方式

在 Zeabur 開源版，服務的**概覽**會列出它的**執行期警示**。沒有 Namespace 的警示，出現在**叢集** → **概覽**的**節點警示**。詳見[執行期警示（Falco）](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/diagnosis/#執行期警示falco)。要把警示送到 Slack 或信箱，詳見[把叢集警示送到通知管道](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/notifications/)。

## 移除

移除會刪除 Falco DaemonSet 和 falcosidekick，偵測隨之停止。警示存在 `PolicyReport` 物件裡，先用 `kubectl get policyreport -A -o yaml` 複製需要的內容。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### Falco 的 Pod 起不來

看 Pod 和 Falco 的日誌。沒有 BTF 的節點載入不了探針：

```sh
kubectl -n falco get pods
kubectl -n falco logs ds/falco -c falco
```

### Zeabur 開源版沒有顯示警示

確認 falcosidekick 有在執行，日誌裡沒有寫入錯誤：

```sh
kubectl -n falco logs deploy/falco-falcosidekick
```
