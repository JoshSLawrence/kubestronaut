#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

gpu=$(ks_node_name node02)
kubectl label node "$gpu" accelerator=gpu --overwrite
kubectl taint node "$gpu" dedicated=gpu:NoSchedule --overwrite
