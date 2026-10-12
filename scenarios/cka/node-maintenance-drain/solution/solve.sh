#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

node=$(ks_node_name node01)
# "drain --force" deletes a pod that no controller owns, so keep its spec.
saved=$(mktemp)
trap 'rm -f "$saved"' EXIT
cat >"$saved" <<'YAML'
apiVersion: v1
kind: Pod
metadata:
  name: scratch
  namespace: maint
  labels:
    app: scratch
spec:
  terminationGracePeriodSeconds: 2
  containers:
    - name: scratch
      image: busybox:1.38.0
      command: [sleep, 1d]
YAML

kubectl drain "$node" --ignore-daemonsets --delete-emptydir-data --force \
  --timeout=180s
kubectl apply -f "$saved"
kubectl -n maint wait pod/scratch --for=condition=Ready --timeout=120s
kubectl -n maint rollout status deployment/web --timeout=120s
