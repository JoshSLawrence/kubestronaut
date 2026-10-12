#!/usr/bin/env bash
set -euo pipefail

# The nine objects metrics-server's manifest creates.
objects="serviceaccount/metrics-server
clusterrole/system:aggregated-metrics-reader
clusterrole/system:metrics-server
rolebinding/metrics-server-auth-reader
clusterrolebinding/metrics-server:system:auth-delegator
clusterrolebinding/system:metrics-server
service/metrics-server
deployment/metrics-server
apiservice/v1beta1.metrics.k8s.io"
while read -r obj; do
  got=$(kubectl -n kube-system get "$obj" \
    -o 'jsonpath={.metadata.labels.app\.kubernetes\.io/part-of}' 2>/dev/null || true)
  if [ "$got" != "monitoring" ]; then
    echo "$obj does not carry the label app.kubernetes.io/part-of=monitoring" >&2
    exit 1
  fi
done <<<"$objects"
