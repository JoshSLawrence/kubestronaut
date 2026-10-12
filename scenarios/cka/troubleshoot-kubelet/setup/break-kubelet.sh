#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Two independent faults: a restart alone must not fix the node.
ks_node node01 sed -i \
  's#/etc/kubernetes/pki/ca.crt#/etc/kubernetes/pki/ca.cert#' \
  /var/lib/kubelet/config.yaml
ks_node node01 systemctl disable --now kubelet
# Wait for NotReady so the first grade fails deterministically.
ks_wait_node node01 NotReady 120
