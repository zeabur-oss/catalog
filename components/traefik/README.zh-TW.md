## 簡介

Traefik Proxy 接收來自網際網路的請求，再依網域轉給你的服務。Zeabur 開源版的公開網域、轉址和密碼都靠它，cert-manager 也需要叢集裡有入口元件。

Traefik 處理 Kubernetes Ingress 和 Gateway API 路由（GatewayClass `traefik`），也讀取 Middleware 這類 Traefik 物件。IngressClass 是叢集預設值。Service 會保留用戶端位址（`externalTrafficPolicy: Local`），所以只有執行 Traefik 的節點會收到流量。多節點叢集裡，Traefik 每個節點跑一個 Pod，包含控制平面節點。

元件刻意關掉了幾項功能：儀表板、由 Chart 建立 Gateway，以及 Traefik 物件參照其他 Namespace 的物件。JSON 存取日誌也只保留幾個欄位，不記錄標頭和查詢字串。

Traefik 會讀取每個 Namespace 的 Service、Secret 和路由，並寫入路由的狀態。

K3s 和 RKE2 自帶 Traefik，Zeabur 開源版會標示為由發行版安裝。要改它的設定，見[修改內建元件的設定](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#修改內建元件的設定)。

## 使用方式

在 Zeabur 開源版的服務**網路**分頁加上公開網域，Zeabur 開源版會替你建立 Ingress 或路由。詳見[加上公開網域與 HTTPS](https://oss.zeabur.com/zh-TW/docs/zeabur/configure/domains/)。

## 移除

移除 Traefik 後，所有入口流量都會中斷。Ingress、HTTPRoute 和 Middleware 物件會留著，但沒有東西處理，CRD 也會留著。cert-manager 需要入口元件，所以 cert-manager 還裝著時，Zeabur 開源版會拒絕移除 Traefik。移除前先確認沒有公開網域還要用它。詳見[移除元件](https://oss.zeabur.com/zh-TW/docs/zeabur/cluster/components/#移除元件)。

## 疑難排解

### 所有網域都沒有回應

執行 `kubectl -n traefik get pods` 和 `kubectl -n traefik get service traefik`。Service 要從負載平衡器拿到外部位址，而且要有 Traefik Pod 跑在會收到流量的節點上。

### GatewayClass 沒有被接受

執行 `kubectl describe gatewayclass traefik`，看條件裡的原因。
