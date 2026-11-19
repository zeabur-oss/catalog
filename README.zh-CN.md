<!-- The language names are written in their own scripts. -->
<!-- vale ZeaburZhCN.Script = NO -->
[English](README.md) | [繁體中文](README.zh-TW.md) | 简体中文
<!-- vale ZeaburZhCN.Script = YES -->

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/cover-zh-CN-dark.avif">
  <img alt="Zeabur Catalog：每个模板都先自动装过、测过，装好就能跑。WordPress、n8n、Ghost、Uptime Kuma、Vaultwarden、Umami、Gitea、Ollama、SearXNG、ntfy 的 logo。" src="docs/images/cover-zh-CN-light.avif">
</picture>

# Zeabur Catalog

在自己的服务器上一键部署 n8n、WordPress、Dify、OpenClaw 等应用，每个模板上架前都先装过、测过。Zeabur 上有模板市场，Zeabur 开源版则有 Zeabur Catalog。

## 在用 Zeabur 开源版吗？

Zeabur 开源版已经内置这份 catalog。在项目中选择**新建** → **应用模板**，选一个应用就能部署。详见 Zeabur 开源版文档的[从应用模板部署](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/templates/)。

从 Zeabur 迁移过来？[Zeabur 模板对照表](docs/zh-CN/get-started/zeabur-com-templates.mdx)列出每个热门 Zeabur 模板在 catalog 中对应的模板。把服务迁移到自己的服务器，请看 [Zeabur 开源版文档](https://oss.zeabur.com/zh-CN/docs/zeabur/)。

如果你用 Zeabur 开源版，读到这里就够了。

## 能帮你做什么

- **上架前先测过**：模板的每次修改合并前，CI 都会把模板装到 Kubernetes 集群上，确认应用有响应。
- **版本固定**：每个模板都固定镜像版本，同一个模板每次装出来都是同一个应用版本。新版模板会随 Zeabur 开源版的升级提供。
- **密码自动生成**：部署时自动生成密码和密钥，存为 Kubernetes Secret，不会写进模板。
- **写明资源需求**：每个模板都写明需要多少 CPU 和内存。应用允许时，以非 root 用户运行。

## 单独使用

不配合 Zeabur 开源版时，catalog 是一份给 Kubernetes 集群用的组件与模板数据，全部经过测试。每次发布都会提供一个签名的包和一份静态 API。组件在整个集群安装一次，所有项目共用；模板装在单个项目中，可以装多次。

- **组件**由集群管理员在每个集群安装一次，例如 Ingress、证书、指标和数据库 operator。每种用途都写明采用哪个上游 chart、哪个版本、哪些配置，以及原因。
- **模板**是项目成员装到单个 Namespace 的应用，例如 n8n、Dify、WordPress 和 PostgreSQL。每个模板都固定镜像，设置资源请求与限制，应用允许时遵循 Restricted Pod Security Standard。模板用 `requires` 列出需要的组件。

catalog 本身是数据。CI 会安装并测试每个组件和模板。每次发布都会提供一个签名的包和一份静态 API，任何工具都能读取，用来生成 `helm install` 命令、Flux 或 Argo CD 清单、k3s 的 `HelmChart` 对象，或安装模板。

[Zeabur 开源版](https://github.com/zeabur-oss/zeabur)会读取这份 catalog，提示集群缺少哪些组件，但不会自己安装组件。

### 文件布局

```text
components/<id>.yaml                      每个上游一个文件；provides 写明它提供的用途
templates/<id>/template.yaml              一个应用：输入、服务、安全、冒烟测试
templates/<id>/manifests.yaml             它的 Kubernetes 对象
templates/<id>/e2e.yaml                       该模板仅测试使用的输入（可选）
schema/v1/                                两种文件的 JSON Schema
config/licenses.yaml                      OSI 批准的 SPDX 标识符，以及只能手动启用的例外
config/kubernetes.yaml                    Kubernetes 版本；e2e 使用的 kind 节点版本
config/budget.yaml                        单节点内存预算（MiB）
config/e2e.yaml                           仅测试使用的值
conformance/                              安装工具和模板客户端的测试用例
measurements/                             空闲时的内存与 CPU，由 CI 写入
tools/                                    `catalog` 命令（Go）
docs/                                     文档
```

两种文件使用同一个外层格式：

```yaml
apiVersion: catalog.zeabur.com/v1
kind: Component                   # 或 Template
metadata:
  name: Traefik Proxy
  license: MIT                    # SPDX
  homepage: https://traefik.io/traefik/
requires: [{capability: certificates}]   # 或 {component: <id>}
```

路径就是 ID：`components/traefik.yaml` 的 ID 是 `traefik`，`templates/n8n/` 的 ID 是 `n8n`。组件在 `provides` 中写明它提供的用途，例如 `provides: {capability: ingress}`。组件 ID 不会与模板 ID 重复。

## 文档

给 Zeabur 开源版用户：

- Zeabur 开源版文档的[从应用模板部署](https://oss.zeabur.com/zh-CN/docs/zeabur/deploy/templates/)和[查看与安装集群组件](https://oss.zeabur.com/zh-CN/docs/zeabur/cluster/components/)
- [Zeabur 模板对照表](docs/zh-CN/get-started/zeabur-com-templates.mdx)
- [模板列表](docs/zh-CN/reference/templates.mdx)

单独使用 catalog：

- [用 catalog 命令部署模板](docs/zh-CN/get-started/deploy-a-template.mdx)
- [组件列表](docs/zh-CN/reference/components.mdx)、[模板列表](docs/zh-CN/reference/templates.mdx)和[各发行版的差异](docs/zh-CN/reference/distributions.mdx)
- [组件文件格式](docs/zh-CN/reference/component-format.mdx)和[模板文件格式](docs/zh-CN/reference/template-format.mdx)
- [模板标准](docs/zh-CN/reference/template-standards.mdx)
- [版本与 API](docs/zh-CN/reference/releases.mdx)
- [添加组件](docs/zh-CN/contribute/add-component.mdx)、[添加模板](docs/zh-CN/contribute/add-template.mdx)和[测试与 CI](docs/zh-CN/contribute/tests.mdx)

## 许可证

工具和原创文件采用 Apache-2.0 许可证。每个上游项目、应用、镜像和 chart 各自保留原有的许可证，写在 `metadata.license` 中。部分项目采用 BUSL-1.1 或 SSPL 这类源码可得（source-available）许可证：这类组件只能手动启用，这类模板则在 `template.yaml` 开头的注释中写明许可证并附上链接。贡献采用 [Developer Certificate of Origin](https://developercertificate.org/)，详见 [CONTRIBUTING.md](CONTRIBUTING.md)。
