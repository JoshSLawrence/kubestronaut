#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_node node01 sed -i 's#pki/ca.cert#pki/ca.crt#' \
  /var/lib/kubelet/config.yaml
ks_node node01 systemctl enable --now kubelet
kubectl wait node "$(ks_node_name node01)" \
  --for=condition=Ready --timeout=120s
kubectl -n ops rollout status deploy/pinned --timeout=120s
