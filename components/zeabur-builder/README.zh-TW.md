## 簡介

Zeabur Builder 用 Dockerfile 或 Railpack，從 Git 儲存庫和上傳的資料夾建置容器映像檔，並把映像檔存在 zot 裡。zot 是隨元件一起安裝的 OCI 映像檔倉庫，所以建置不需要外部倉庫的帳號。Zeabur 開源版要從 Git 部署服務，就需要它；沒有它，服務只能從映像檔部署。

Builder 會為每個要建置的 Namespace 執行一個 buildkitd。元件用 cert-manager 建立一個 CA，替 zot 簽發憑證。推送映像檔需要 Builder 簽發的短效 token。NetworkPolicy 只讓 Builder、buildkitd 和建置 Pod 連到 zot。zot 每個倉庫保留最近的 20 個 tag，映像檔存在 20Gi 的磁碟區。

Builder 會讀取、建立、更新和刪除每個 Namespace 的 Secret（Git 憑證和它們的複本），並管理那裡的 Build、BuildRun 和 Job。每個 buildkitd 都有特權，並搭配 user namespace 和對外連線規則執行。

## 安裝前準備

- 先安裝 cert-manager，由它簽發 zot 的憑證。
- 節點用輸入項 `clusterIp` 的固定位址從 zot 拉映像檔。這是叢集 Service CIDR 裡一個沒人用的位址；留空時 Zeabur 會自動挑一個。在 K3s、RKE2 和 KubeSolo 上，`zot-node` DaemonSet 會讓每個節點改連這個位址，並給節點 `zot-tls` Secret 裡的 CA。其他發行版要自己在每個節點設定 `zot.zot.svc:5000` 指向這個位址的映像檔倉庫鏡像，並信任這個 CA。

## 使用方式

在 Zeabur 開源版專案裡選**新增** → **從 Git 部署**，輸入儲存庫。詳見[從 Git 建置並部署服務](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/git/)，同一頁也說明上傳資料夾。建置完成的映像檔由 Keel 部署，所以也要安裝 Keel。

### 啟用 AI 建置器

AI 建置器（`builderRef: agent`）會請模型替沒有 Dockerfile、Railpack 也建置不了的儲存庫寫一份 Dockerfile。模型會讀取儲存庫的檔案，這些檔案會傳給模型供應商，而且每寫一份 Dockerfile 都會用掉 token。之後的建置沿用這份 Dockerfile，不再呼叫模型。這個建置器預設關閉，詳見[用 AI 建置器建置](https://oss.zeabur.com/zh-TW/docs/builder/builds/ai-builder/)。

1. 決定 run 怎麼登入模型 API，並在每個有 Build 使用 AI 建置器的 Namespace 建立它需要的東西：放 API 金鑰的 Secret；用權杖交換時，則是 `agentAuth` 指定的 ServiceAccount。
2. 設定元件的輸入項：`agentEnabled` 設為 `true`，再設定 `agentProvider`（供應商在 Rig 裡的名稱，例如 `anthropic` 或 `openai`）、`agentModel` 和 `agentAuth`。只有要把要求送到代理伺服器或自己架的伺服器時，才設定 `agentBaseUrl`。設定好後，元件會安裝名為 `agent` 的 ClusterBuilder。
3. 把 Build 的 `spec.builderRef` 設為 `agent`。Zeabur 開源版的介面還不能選 AI 建置器（[zeabur-oss/zeabur#1021](https://github.com/zeabur-oss/zeabur/issues/1021)）。

`agentAuth` 就是 ClusterBuilder 的 `spec.agent.model.auth`。API 金鑰放在 Secret `model` 的 key `anthropic` 時：

```yaml
type: apiKey
apiKey: {name: model, key: anthropic}
```

Anthropic 和 OpenAI 也可以改用服務帳戶權杖交換，這樣每個 Namespace 都不必存 API 金鑰：設定 `type: anthropicFederation` 或 `type: openaiFederation`。其他 OAuth 2.0 權杖端點用 `type: tokenExchange`。以搭配 Microsoft Entra ID 的 Azure OpenAI 為例：

```yaml
type: tokenExchange
tokenExchange:
  serviceAccountName: builder-agent
  audience: api://AzureADTokenExchange
  url: https://login.microsoftonline.com/<tenant>/oauth2/v2.0/token
  subjectField: client_assertion
  form:
    grant_type: client_credentials
    client_id: <application client ID>
    client_assertion_type: urn:ietf:params:oauth:client-assertion-type:jwt-bearer
    scope: https://cognitiveservices.azure.com/.default
```

ClusterBuilder 不是 Secret，所以 `form` 裡不要放任何機密。上面連結的頁面列出每種類型的欄位，也說明怎麼向供應商註冊叢集。

## 移除

移除 Zeabur Builder 後，所有建置都會停止。`zot` 的磁碟區和裡面所有的映像檔會留著，CRD 和所有 Build 也會留著。

移除前先確認沒有服務還從 zot 拉映像檔。zot 移除後，重新啟動或搬到其他節點的 Pod 就拉不到映像檔。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### zot 無法啟動

zot 會等 Secret `zot-tls` 和 `zot-token-signer` 出現。執行 `kubectl -n zot get certificate,pods`。Certificate 沒有就緒，就是 cert-manager 的問題。

### 服務拉不到映像檔

Pod 停在 `ImagePullBackOff`。執行 `kubectl describe pod <name>` 看事件。在 K3s、RKE2 和 KubeSolo 上，確認那個節點有一個在跑的 `zot-node` Pod，而且 `/var/lib/rancher/k3s/agent/etc/containerd/certs.d/zot.zot.svc:5000/hosts.toml`（RKE2 是 `rancher/rke2`，KubeSolo 是 `/var/lib/kubesolo/containerd/registry/`）寫的是 `clusterIp` 的位址。

### AI 建置器的 run 停在 `ModelKeyMissing` 等待

Build 所在的 Namespace 少了 `agentAuth` 指定的 Secret、Secret 裡的 key 或 ServiceAccount。執行 `kubectl -n <namespace> get buildrun` 看訊息，建立它說缺少的東西。補好後 run 會自己開始。
