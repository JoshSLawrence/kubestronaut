#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# kind kubelets serve self-signed certificates, so metrics-server needs
# --kubelet-insecure-tls to scrape them.
ks_log "installing metrics-server"
kubectl apply -f files/metrics-server.yaml
kubectl -n kube-system patch deployment metrics-server --type json -p \
  '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
kubectl -n kube-system rollout status deployment/metrics-server --timeout=240s
kubectl wait --for=condition=Available apiservice/v1beta1.metrics.k8s.io --timeout=240s

# The first scrape takes a while: wait until the numbers are there so the
# learner's HPA is not blamed for a slow start.
for _ in $(seq 60); do
  if kubectl top nodes >/dev/null 2>&1; then
    exit 0
  fi
  sleep 3
done
echo "metrics-server did not serve node metrics within 180s" >&2
exit 1
