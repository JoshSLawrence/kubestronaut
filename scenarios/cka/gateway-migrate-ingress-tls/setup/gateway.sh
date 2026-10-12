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
