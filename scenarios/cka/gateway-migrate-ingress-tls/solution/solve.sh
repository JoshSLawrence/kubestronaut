#!/usr/bin/env bash
set -euo pipefail

kubectl apply -f - <<'YAML'
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
YAML

kubectl -n web wait --for=condition=Programmed --timeout=60s gateway/web-gateway
kubectl -n web delete ingress web
