## 簡介

Trivy Operator 掃描執行中工作負載的映像檔，找出已知弱點。它也會檢查工作負載的設定，並找出外洩的機密。結果寫成報告，放在該工作負載所在的 Namespace：`VulnerabilityReport`、`ConfigAuditReport` 和 `ExposedSecretReport`。

想知道叢集裡有哪些映像檔有弱點時，就裝它。掃描 Job 會用掉不少 CPU 和記憶體，所以這個元件是選配。這個元件只列出有修補版本的弱點。單節點時，一次掃描一個映像檔，只回報 MEDIUM、HIGH 和 CRITICAL 的結果，並略過 RBAC 檢查。

Operator 讀取每個 Namespace 的工作負載、Pod、Service、Ingress 和 RBAC 物件，執行掃描 Job 並寫入報告，不會讀取所有 Secret。基礎設施掃描、合規掃描和 SBOM 報告都是關閉的。

## 使用方式

Trivy Operator 沒有 Zeabur 開源版畫面。等第一批掃描完成後，用 kubectl 讀報告。

```sh
kubectl get vulnerabilityreports -A -o wide
kubectl get configauditreports -A -o wide
```

報告欄位和設定詳見 [Trivy Operator 文件](https://aquasecurity.github.io/trivy-operator/)。

## 移除

移除會刪除 operator。CRD 會留著，已經寫好的報告也會留著，不想要時自己刪除。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 沒有出現報告

看 operator 的日誌，並找出失敗的掃描 Job：

```sh
kubectl -n trivy-system logs deploy/trivy-operator
kubectl -n trivy-system get jobs,pods
```
