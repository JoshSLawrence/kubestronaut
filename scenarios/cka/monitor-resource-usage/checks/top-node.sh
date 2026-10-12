#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

got=$(ks_node controlplane cat /root/top-node.txt 2>/dev/null | tr -d '[:space:]' || true)
if [ -z "$got" ]; then
  echo "/root/top-node.txt is missing or empty" >&2
  exit 1
fi
# Judged against the metrics as they are now, not a fixed answer.
want=$(kubectl top nodes --no-headers --sort-by=memory | head -n 1 | awk '{print $1}')
if [ "$got" != "$want" ]; then
  echo "/root/top-node.txt does not name the node using the most memory" >&2
  exit 1
fi
