# Solution

The key file and certificate are on `controlplane`, so create the Secret there:

```bash
kubestronaut ssh controlplane
kubectl -n shop create secret tls shop-tls \
  --cert=/opt/certs/shop.crt --key=/opt/certs/shop.key
```

Then create the Ingress. The class is not the default, so
`ingressClassName: traefik` is required, or Traefik ignores the object. Both
paths use `pathType: Prefix`: `/api` and `/api/v1/items` go to `api`, and
everything else falls through to `/`, the `web` rule. `Exact` would only match
`/api` itself. Naming the host keeps other
hosts unrouted, which a rule without a host or a `defaultBackend` would not.

```bash
kubectl create ingress shop -n shop --class=traefik \
  --rule='shop.example.local/api*=api:8080,tls=shop-tls' \
  --rule='shop.example.local/*=web:80,tls=shop-tls'
```

The same Ingress as a manifest:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: shop
  namespace: shop
spec:
  ingressClassName: traefik
  tls:
    - hosts: [shop.example.local]
      secretName: shop-tls
  rules:
    - host: shop.example.local
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service: {name: api, port: {number: 8080}}
          - path: /
            pathType: Prefix
            backend:
              service: {name: web, port: {number: 80}}
```

Test it from `controlplane` with the node port and a `Host` header:

```bash
curl -H 'Host: shop.example.local' http://localhost:30080/api/v1/items
```
