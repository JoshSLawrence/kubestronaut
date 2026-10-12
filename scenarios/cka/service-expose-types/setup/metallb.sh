#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_log "installing MetalLB"
kubectl apply --server-side -f files/metallb-native.yaml
kubectl -n metallb-system rollout status deployment/controller --timeout=180s
kubectl -n metallb-system rollout status daemonset/speaker --timeout=180s

# The address pool is carved from the kind network's /16 (the nodes' own
# subnet), high enough to stay clear of the addresses Docker hands out.
ips=$(kubectl get node "$KS_NODE_CONTROLPLANE" \
  -o 'jsonpath={.status.addresses[?(@.type=="InternalIP")].address}')
ip=${ips%% *}
IFS=. read -r a b _ _ <<<"$ip"
if ! [[ $a.$b =~ ^[0-9]+\.[0-9]+$ ]]; then
  ks_log "cannot derive an address pool from the node address '$ip'"
  exit 1
fi

pool=$(mktemp)
trap 'rm -f "$pool"' EXIT
cat >"$pool" <<YAML
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kind
  namespace: metallb-system
spec:
  addresses:
    - $a.$b.250.200-$a.$b.250.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kind
  namespace: metallb-system
YAML

# The admission webhook can lag behind the rollout, so retry the apply.
for _ in $(seq 30); do
  if kubectl apply -f "$pool"; then
    exit 0
  fi
  sleep 2
done
ks_log "MetalLB did not accept its address pool within 60s"
exit 1
