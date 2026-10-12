#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_node controlplane helm install edge /opt/charts/traefik-41.7.1.tgz \
  --namespace ingress --create-namespace --skip-crds \
  --set deployment.replicas=2 \
  --set service.spec.type=NodePort \
  --set ports.web.nodePort=30080 \
  --set providers.kubernetesCRD.enabled=false \
  --wait --timeout 4m

ks_node controlplane helm rollback storefront 1 --namespace shop --wait --timeout 3m
