#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# "--previous" only works while the next instance runs; between restarts the
# plain log is the one that just exited. Every instance logs the same last line.
line=""
for _ in $(seq 30); do
  line=$(kubectl -n orders logs deployment/orders --previous 2>/dev/null | tail -n1 || true)
  case "$line" in FATAL*) break ;; esac
  line=$(kubectl -n orders logs deployment/orders 2>/dev/null | tail -n1 || true)
  case "$line" in FATAL*) break ;; esac
  sleep 2
done
printf '%s\n' "$line" | ks_node controlplane sh -c 'cat > /root/orders-error.txt'

kubectl -n orders patch deployment orders --type=json -p '[
  {"op":"add","path":"/spec/template/spec/containers/0/env/-","value":
    {"name":"DB_HOST","valueFrom":{"configMapKeyRef":
      {"name":"orders-config","key":"database-host"}}}}]'
kubectl -n orders rollout status deployment/orders --timeout=120s
