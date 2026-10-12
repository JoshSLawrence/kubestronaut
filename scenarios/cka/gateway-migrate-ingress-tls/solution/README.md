# Solution

The Gateway takes over what the Ingress did: the listener carries the host
name, the port of Traefik's HTTPS entry point (8443, which is what node port
30443 forwards to), and the Secret the Ingress used under `tls`. The route
attaches to the Gateway with `parentRefs` and sends the host's traffic to the
Service:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: web-gateway
  namespace: web
spec:
  gatewayClassName: traefik
  listeners:
    - name: https
      protocol: HTTPS
      port: 8443
      hostname: web.example.local
      tls:
        mode: Terminate
        certificateRefs:
          - name: web-tls
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: web-route
  namespace: web
spec:
  parentRefs:
    - name: web-gateway
  hostnames: [web.example.local]
  rules:
    - backendRefs:
        - name: web
          port: 80
```

The listener port must be the controller's entry point port, not the node
port. A listener without `hostname` would answer every host. Check that the
Gateway is `Programmed` and the route `Accepted`, test with `curl -k
--resolve`, and only then delete the Ingress:

```bash
kubectl -n web delete ingress web
```

While the Ingress exists, it keeps answering the same requests, so a working
request alone does not prove the migration.
