#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_log "installing Traefik"
kubectl apply -f files/traefik.yaml
kubectl -n traefik rollout status deployment/edge-traefik --timeout=180s

ks_log "creating the certificate on controlplane"
ks_node controlplane bash -c 'mkdir -p /opt/certs &&
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
    -subj /CN=shop.example.local \
    -addext subjectAltName=DNS:shop.example.local \
    -keyout /opt/certs/shop.key -out /opt/certs/shop.crt 2>/dev/null'
fp=$(ks_node controlplane openssl x509 -in /opt/certs/shop.crt -noout -fingerprint -sha256)
ks_answer_put shop-cert "$fp"
