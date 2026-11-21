## 簡介

Zeabur Builder 用 Dockerfile 或 Railpack，從 Git 儲存庫和上傳的資料夾建置容器映像檔，並把映像檔存在 zot 裡。zot 是隨元件一起安裝的 OCI 映像檔倉庫，所以建置不需要外部倉庫的帳號。Zeabur 開源版要從 Git 部署服務，就需要它；沒有它，服務只能從映像檔部署。

Builder 會為每個要建置的 Namespace 執行一個 buildkitd。元件用 cert-manager 建立一個 CA，替 zot 簽發憑證。推送映像檔需要 Builder 簽發的短效 token。NetworkPolicy 只讓 Builder、buildkitd 和建置 Pod 連到 zot。zot 每個倉庫保留最近的 20 個 tag，映像檔存在 20Gi 的磁碟區。

Builder 會讀取、建立、更新和刪除每個 Namespace 的 Secret（Git 憑證和它們的複本），並管理那裡的 Build、BuildRun 和 Job。每個 buildkitd 都有特權，並搭配 user namespace 和對外連線規則執行。

## 安裝前準備

- 先安裝 cert-manager，由它簽發 zot 的憑證。
- 節點用輸入項 `clusterIp` 的固定位址從 zot 拉映像檔。這是叢集 Service CIDR 裡一個沒人用的位址；留空時 Zeabur 會自動挑一個。在 K3s、RKE2 和 KubeSolo 上，`zot-node` DaemonSet 會讓每個節點改連這個位址，並給節點 `zot-tls` Secret 裡的 CA。其他發行版要自己在每個節點設定 `zot.zot.svc:5000` 指向這個位址的映像檔倉庫鏡像，並信任這個 CA。

## 使用方式

在 Zeabur 開源版專案裡選**新增** → **從 Git 部署**，輸入儲存庫。詳見[從 Git 建置並部署服務](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/git/)，同一頁也說明上傳資料夾。建置完成的映像檔由 Keel 部署，所以也要安裝 Keel。

## 移除

移除 Zeabur Builder 後，所有建置都會停止。`zot` 的磁碟區和裡面所有的映像檔會留著，CRD 和所有 Build 也會留著。

移除前先確認沒有服務還從 zot 拉映像檔。zot 移除後，重新啟動或搬到其他節點的 Pod 就拉不到映像檔。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### zot 無法啟動

zot 會等 Secret `zot-tls` 和 `zot-token-signer` 出現。執行 `kubectl -n zot get certificate,pods`。Certificate 沒有就緒，就是 cert-manager 的問題。

### 服務拉不到映像檔

Pod 停在 `ImagePullBackOff`。執行 `kubectl describe pod <name>` 看事件。在 K3s、RKE2 和 KubeSolo 上，確認那個節點有一個在跑的 `zot-node` Pod，而且 `/var/lib/rancher/k3s/agent/etc/containerd/certs.d/zot.zot.svc:5000/hosts.toml`（RKE2 是 `rancher/rke2`，KubeSolo 是 `/var/lib/kubesolo/containerd/registry/`）寫的是 `clusterIp` 的位址。
