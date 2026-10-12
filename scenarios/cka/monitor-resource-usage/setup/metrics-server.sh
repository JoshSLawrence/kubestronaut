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
