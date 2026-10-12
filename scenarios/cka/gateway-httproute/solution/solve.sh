#!/usr/bin/env bash
set -euo pipefail

kubectl label namespace shop shared-gateway-access=true

kubectl apply -f - <<'YAML'
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api
  namespace: shop
spec:
  parentRefs:
    - name: web-gw
      namespace: infra
  hostnames: [api.example.local]
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api
          headers:
            - name: x-canary
              value: "true"
      backendRefs:
        - name: api-v2
          port: 8080
    - matches:
        - path:
            type: PathPrefix
            value: /api
      backendRefs:
        - name: api-v1
          port: 8080
          weight: 90
        - name: api-v2
          port: 8080
          weight: 10
YAML
