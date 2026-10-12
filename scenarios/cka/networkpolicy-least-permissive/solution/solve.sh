#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_node controlplane kubectl apply -f /root/netpol/policy-b.yaml
