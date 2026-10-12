#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

pod=$(kubectl top pods -n metrics -l app=batch --no-headers --sort-by=cpu | head -n 1 | awk '{print $1}')
node=$(kubectl top nodes --no-headers --sort-by=memory | head -n 1 | awk '{print $1}')
printf '%s\n' "$pod" | ks_node controlplane sh -c 'cat > /root/top-pod.txt'
printf '%s\n' "$node" | ks_node controlplane sh -c 'cat > /root/top-node.txt'
