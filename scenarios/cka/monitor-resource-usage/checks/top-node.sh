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
top=$(kubectl top nodes --no-headers --sort-by=memory 2>/dev/null) || {
  echo "kubectl top nodes failed: metrics are not available yet" >&2
  exit 1
}
want=$(awk 'NR == 1 {print $1}' <<<"$top")
if [ "$got" != "$want" ]; then
  echo "/root/top-node.txt does not name the node using the most memory" >&2
  exit 1
fi
