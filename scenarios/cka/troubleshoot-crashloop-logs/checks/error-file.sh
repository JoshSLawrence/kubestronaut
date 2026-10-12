#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

content=$(ks_node controlplane cat /root/orders-error.txt 2>/dev/null) ||
  {
    echo "/root/orders-error.txt does not exist on controlplane" >&2
    exit 1
  }
# Ignore surrounding whitespace and a Windows line ending, nothing else.
content=$(printf '%s' "$content" | tr -d '\r' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
if ! ks_answer_check error-line "$content"; then
  echo "/root/orders-error.txt does not hold only the last line of the previous instance's log" >&2
  exit 1
fi
