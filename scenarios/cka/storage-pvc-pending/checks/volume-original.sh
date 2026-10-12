#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

uid=$(kubectl get pv archive-pv -o 'jsonpath={.metadata.uid}')
if ! ks_answer_check pv-uid "$uid"; then
  echo "archive-pv is not the original volume" >&2
  exit 1
fi
