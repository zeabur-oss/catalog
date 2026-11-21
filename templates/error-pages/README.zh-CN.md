## 简介

错误页面提供维护页面，以及状态码 404、502、503、504 的页面。它是一个小型的 [nginx](https://nginx.org/) 服务器，页面放在 ConfigMap 里。

这个模板没有域名，也不用登录。Zeabur 开源版会把项目域名的维护页面和错误页面指向这个服务。同一个命名空间的其他工作负载也能使用。

## 开始使用

1. 在 Zeabur 开源版打开 `error-pages` 服务。**概览**显示副本已就绪，服务就启动好了。它没有公开域名，**网络**标签页会显示它的私有地址。
2. 其他工作负载通过 Service `error-pages` 连接。端口 8080 在 `http://error-pages:8080/<状态码>.html` 提供各个页面，例如 `http://error-pages:8080/502.html`。端口 8081 对每个请求都返回维护页面和状态码 `503`。
3. 想从自己的电脑查看页面，先开一条隧道再请求。Zeabur 开源版没有对应的界面，请用 kubectl：

   ```sh
   kubectl -n <namespace> port-forward service/error-pages 8080:8080
   curl http://localhost:8080/502.html
   ```

这个模板没有需要读取的机密。

## 配置

### 在 Traefik 的 errors 中间件使用这些页面

在同一个命名空间创建 `Middleware`，把错误响应发到端口 8080，并按状态码获取对应的页面：

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: error-pages
spec:
  errors:
    status:
      - "404"
      - "502-504"
    query: /{status}.html
    service:
      name: error-pages
      port: 8080
```

详见 Traefik 文档的 [Errors](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/errorpages/)。

### 修改页面文字

页面是 ConfigMap `error-pages` 的 `404.html`、`502.html`、`503.html`、`504.html` 和 `maintenance.html`。在项目的**对象**页面打开 ConfigMap `error-pages`，切到 YAML 视图，点击**编辑**，修改页面后检查并应用。Kubernetes 大约一分钟内会更新挂载的文件，nginx 每次请求都会重新读取页面，所以不用重启任何东西。
