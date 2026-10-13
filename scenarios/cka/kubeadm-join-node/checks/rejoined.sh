#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

uid=$(kubectl get node "$(ks_node_name node01)" -o 'jsonpath={.metadata.uid}' 2>/dev/null || true)
if [ -z "$uid" ]; then
  echo "node01 is not registered in the cluster" >&2
  exit 1
fi
if ks_answer_check node01-uid "$uid"; then
  echo "node01 is still the node object from before the reset" >&2
  exit 1
fi
