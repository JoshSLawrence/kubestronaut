#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_log "installing the Gateway API CRDs"
kubectl apply --server-side -f files/gateway-api.yaml
kubectl wait --for=condition=Established --timeout=60s \
  crd/gatewayclasses.gateway.networking.k8s.io \
  crd/gateways.gateway.networking.k8s.io \
  crd/httproutes.gateway.networking.k8s.io

ks_log "installing Traefik"
kubectl apply -f files/traefik.yaml
kubectl -n traefik rollout status deployment/edge-traefik --timeout=180s
kubectl wait --for=condition=Accepted --timeout=120s gatewayclass/traefik

ks_log "creating the Gateway"
kubectl apply -f - <<'YAML'
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: web-gw
  namespace: infra
spec:
  gatewayClassName: traefik
  listeners:
    - name: http
      protocol: HTTP
      port: 8000
      allowedRoutes:
        namespaces:
          from: Selector
          selector:
            matchLabels:
              shared-gateway-access: "true"
YAML
kubectl -n infra wait --for=condition=Accepted --timeout=120s gateway/web-gw

uid=$(kubectl -n infra get gateway web-gw -o 'jsonpath={.metadata.uid}')
ks_answer_put gateway-uid "$uid"
