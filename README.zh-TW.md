<!-- The language names are written in their own scripts. -->
<!-- vale ZeaburZhTW.Script = NO -->
[English](README.md) | 繁體中文 | [简体中文](README.zh-CN.md)
<!-- vale ZeaburZhTW.Script = YES -->

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/cover-zh-TW-dark.avif">
  <img alt="Zeabur Catalog：每個範本都先自動裝過、測過，裝好就能跑。WordPress、n8n、Ghost、Uptime Kuma、Vaultwarden、Umami、Gitea、Ollama、SearXNG、ntfy 的 logo。" src="docs/images/cover-zh-TW-light.avif">
</picture>

# Zeabur Catalog

在自己的伺服器上一鍵部署 n8n、WordPress、Dify、OpenClaw 等應用程式，每個範本上架前都先裝過、測過。Zeabur 上有範本市集，Zeabur 開源版則有 Zeabur Catalog。

## 在用 Zeabur 開源版嗎？

Zeabur 開源版已經內建這份 catalog。在專案選擇**新增** → **應用程式範本**，挑一個應用程式就能部署。詳情見 Zeabur 開源版文件的[從應用程式範本部署](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/templates/)。

從 Zeabur 搬過來？[Zeabur 範本對照表](docs/zh-TW/get-started/zeabur-com-templates.mdx)列出每個熱門 Zeabur 範本在 catalog 裡對應的範本。把服務搬到自己的伺服器，請看 [Zeabur 開源版文件](https://oss.zeabur.com/zh-TW/docs/zeabur/)。

使用 Zeabur 開源版的話，讀到這裡就夠了。

## 能幫你做什麼

- **上架前先測過**：範本的每次修改合併前，CI 都會把範本裝到 Kubernetes 叢集上，確認應用程式有回應。
- **版本固定**：每個範本都固定映像檔版本，同一個範本每次裝出來都是同一個應用程式版本。新版範本會隨 Zeabur 開源版的升級提供。
- **密碼自動產生**：部署時會自動產生密碼和金鑰，存成 Kubernetes Secret，不會寫在範本裡。
- **寫明資源需求**：每個範本都寫明需要多少 CPU 和記憶體。應用程式允許時，以非 root 使用者執行。

## 單獨使用

不搭配 Zeabur 開源版時，catalog 是一份給 Kubernetes 叢集用的元件與範本資料，全部經過測試。每次發行都會提供一個已簽章的套件包和一份靜態 API。元件在整個叢集安裝一次，所有專案共用；範本則裝在單一專案裡，可以裝很多次。

- **元件**由叢集管理員在每個叢集安裝一次，例如 Ingress、憑證、指標和資料庫 operator。每種用途都會寫明採用的上游 chart、版本、設定，以及選擇的原因。
- **範本**是專案成員裝到單一 Namespace 的應用程式，例如 n8n、Dify、WordPress 和 PostgreSQL。每個範本都會固定映像檔，設定資源要求與上限，應用程式允許時遵守 Restricted Pod Security Standard。範本用 `requires` 列出需要的元件。

catalog 本身是資料。CI 會安裝並測試每個元件和範本。每次發行都會提供一個已簽章的套件包和一份靜態 API，任何工具都能讀取，用來產生 `helm install` 指令、Flux 或 Argo CD 清單、k3s 的 `HelmChart` 物件，或安裝範本。

[Zeabur 開源版](https://github.com/zeabur-oss/zeabur)會讀取這份 catalog，提示叢集缺少哪些元件，但不會自己安裝元件。

### 檔案配置

```text
components/<id>/component.yaml            一個上游；用途寫在 provides
components/<id>/README*.md                它的 README：README.md、README.zh-TW.md、README.zh-CN.md
templates/<id>/template.yaml              一個應用程式：輸入、服務、安全性、煙霧測試
templates/<id>/manifests.yaml             它的 Kubernetes 物件
templates/<id>/e2e.yaml                       該範本只在測試使用的輸入（選用）
schema/v1/                                兩種檔案的 JSON Schema
config/licenses.yaml                      OSI 核准的 SPDX 識別碼，以及只能手動啟用的例外
config/kubernetes.yaml                    Kubernetes 版本；e2e 使用的 kind 節點版本
config/budget.yaml                        單節點記憶體預算（MiB）
config/e2e.yaml                           只在測試使用的值
conformance/                              安裝工具和範本用戶端的測試案例
measurements/                             閒置時的記憶體與 CPU，由 CI 寫入
tools/                                    `catalog` 指令（Go）
docs/                                     文件
```

兩種檔案都使用同一個外層格式：

```yaml
apiVersion: catalog.zeabur.com/v1
kind: Component                   # 或 Template
metadata:
  name: Traefik Proxy
  license: MIT                    # SPDX
  homepage: https://traefik.io/traefik/
requires: [{capability: certificates}]   # 或 {component: <id>}
```

目錄名稱就是 ID：`components/traefik/` 的 ID 是 `traefik`，`templates/n8n/` 的 ID 是 `n8n`。元件在 `provides` 寫明它提供的用途，例如 `provides: {capability: ingress}`。元件 ID 不會和範本 ID 重複。

## 文件

給 Zeabur 開源版使用者：

- Zeabur 開源版文件的[從應用程式範本部署](https://oss.zeabur.com/zh-TW/docs/zeabur/deploy/templates/)和[檢視與安裝叢集元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/)
- [Zeabur 範本對照表](docs/zh-TW/get-started/zeabur-com-templates.mdx)
- [範本清單](docs/zh-TW/reference/templates.mdx)

單獨使用 catalog：

- [用 catalog 指令部署範本](docs/zh-TW/get-started/deploy-a-template.mdx)
- [元件清單](docs/zh-TW/reference/components.mdx)、[範本清單](docs/zh-TW/reference/templates.mdx)和[各發行版的差異](docs/zh-TW/reference/distributions.mdx)
- [元件檔案格式](docs/zh-TW/reference/component-format.mdx)和[範本檔案格式](docs/zh-TW/reference/template-format.mdx)
- [範本標準](docs/zh-TW/reference/template-standards.mdx)
- [版本與 API](docs/zh-TW/reference/releases.mdx)
- [新增元件](docs/zh-TW/contribute/add-component.mdx)、[新增範本](docs/zh-TW/contribute/add-template.mdx)和[測試與 CI](docs/zh-TW/contribute/tests.mdx)

## 授權

工具和原創檔案採用 Apache-2.0 授權。每個上游專案、應用程式、映像檔和 chart 各自保留原本的授權，寫在 `metadata.license`。部分專案採用 BUSL-1.1 或 SSPL 這類原始碼可得（source-available）授權：這類元件只能手動啟用，這類範本則在 `template.yaml` 開頭的註解寫明授權並附上連結。貢獻採用 [Developer Certificate of Origin](https://developercertificate.org/)，詳見 [CONTRIBUTING.md](CONTRIBUTING.md)。
