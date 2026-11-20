## 简介

[NGINX](https://nginx.org/) 是网页服务器，也能当反向代理。

这个模板在你的域名后面运行一个 NGINX。它对每个请求都只回一行纯文本，不提供任何文件。要改变它的行为，请编辑 ConfigMap 里的配置。

## 开始使用

1. 在 Zeabur 打开 `nginx` 服务。**概览**显示副本已就绪，服务就启动好了。域名在**网络**标签页的**公开域名**中，就是你在安装表单填的域名，例如 `nginx.example.com`。
2. 页面会显示 `Hello from Nginx deployed on Zeabur!` 这行文字。不需要登录，模板也不会创建任何机密。
3. 要提供其他内容，请见“配置”。

## 配置

### 改变 NGINX 的行为

整份 `nginx.conf` 都在 ConfigMap `nginx` 里。NGINX 只在启动时读取它。打开项目的**对象**页面，打开 ConfigMap `nginx`，切换到 YAML 视图，点击**编辑**修改，检查后应用。再点击 `nginx` 服务页面顶部的**重启**。

根文件系统是只读的。请保留 `listen 8080`、`pid` 那一行，以及指向 `/tmp` 的 `*_temp_path` 那几行。详见 NGINX 文档的 [Beginner's guide](https://nginx.org/en/docs/beginners_guide.html)。

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

配置有错时，NGINX 不会启动。新的 Pod 就绪之前，旧的 Pod 会继续提供服务，所以网站不会中断。打开 `nginx` 服务的**日志**标签页，用顶部的选择器选新的副本，看 NGINX 拒绝的那一行。在**对象**页面修正 ConfigMap，再点击一次**重启**。
