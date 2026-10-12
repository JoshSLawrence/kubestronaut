#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

join=$(ks_node controlplane kubeadm token create --ttl 2h --print-join-command)
ks_node controlplane sh -c "printf '%s\n' '$join' > /root/node01-join.sh"
ks_node node01 sh -c "$join"
ks_wait_node node01 Ready 180
kubectl -n edge rollout status deployment/pinned --timeout=180s
