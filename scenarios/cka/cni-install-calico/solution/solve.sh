#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_node controlplane kubectl apply -f /opt/cni/calico.yaml
kubectl wait --for=condition=Ready node --all --timeout=300s
kubectl -n kube-system rollout status deployment/coredns --timeout=180s
kubectl -n net-test wait --for=condition=Ready pod --all --timeout=180s
kubectl -n secure wait --for=condition=Ready pod --all --timeout=180s
