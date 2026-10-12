#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_log "placing the charts on controlplane"
ks_copy_to controlplane files/charts /opt/charts

ks_log "installing storefront, then upgrading it to a version that cannot start"
ks_node controlplane helm install storefront /opt/charts/storefront \
  --namespace shop --create-namespace --wait --timeout 3m
ks_node controlplane helm upgrade storefront /opt/charts/storefront \
  --namespace shop --set image.tag=2.61-missing

# Wait for the failed rollout to show, so the first grade is stable.
for _ in $(seq 60); do
  reason=$(kubectl -n shop get pods \
    -o 'jsonpath={.items[*].status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || true)
  case "$reason" in
    *ImagePullBackOff* | *ErrImagePull*) exit 0 ;;
  esac
  sleep 3
done
echo "the broken revision did not show an image error within 180s" >&2
exit 1
