#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

got=$(ks_node controlplane cat /root/top-pod.txt 2>/dev/null | tr -d '[:space:]' || true)
if [ -z "$got" ]; then
  echo "/root/top-pod.txt is missing or empty" >&2
  exit 1
fi
if ! ks_answer_check top-pod "$got"; then
  echo "/root/top-pod.txt does not name the busiest app=batch pod" >&2
  exit 1
fi
