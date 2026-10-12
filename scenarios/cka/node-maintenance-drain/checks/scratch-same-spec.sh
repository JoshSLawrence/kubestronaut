#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

spec=$(kubectl -n maint get pod scratch -o \
  'jsonpath={.spec.containers[0].image}{"|"}{.spec.containers[0].command}' 2>/dev/null || true)
if ! ks_answer_check scratch-spec "$spec"; then
  echo "pod maint/scratch is missing or has a different image or command" >&2
  exit 1
fi
