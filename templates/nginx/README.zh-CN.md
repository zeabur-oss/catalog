## 简介

[NGINX](https://nginx.org/) 是网页服务器，也能当反向代理。

这个模板在你的域名后面运行一个 NGINX。它对每个请求都只回一行纯文本，不提供任何文件。要改变它的行为，请编辑 ConfigMap 里的配置。

## 开始使用

1. 等服务显示为运行中，打开它的域名，例如 `https://nginx.example.com`。
2. 页面会显示 `Hello from Nginx deployed on Zeabur!` 这行文字。不需要登录，模板也不会创建任何机密。
3. 要提供其他内容，请见“配置”。

## 配置

### 改变 NGINX 的行为

完整的 `nginx.conf` 放在 ConfigMap `nginx` 里。Pod 只在启动时读取它，所以编辑后要重启 Deployment：

```sh
kubectl -n <namespace> edit configmap nginx
kubectl -n <namespace> rollout restart deployment/nginx
```

根文件系统是只读的。请保留 `listen 8080`、`pid` 那一行，以及指向 `/tmp` 的 `*_temp_path` 各行。详见 NGINX 文档的 [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html)。

### 把请求转给另一个服务

把 `location /` 里的 `return` 那一行，换成指向同一个命名空间里某个 Service 的 `proxy_pass`：

```nginx
location / {
  proxy_pass http://my-app:3000;
}
```

详见 [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html) 的 "Setting Up a Simple Proxy Server"。

## 故障排除

### 编辑后新的 Pod 起不来

配置有错时，NGINX 会拒绝启动。新的 Pod 就绪之前，旧的 Pod 会继续提供服务，所以网站不会中断。请查看新 Pod 的日志，找出 NGINX 拒绝的那一行，修正 ConfigMap 后再重启 Deployment：

```sh
kubectl -n <namespace> get pods
kubectl -n <namespace> logs <Pod 名称>
```
