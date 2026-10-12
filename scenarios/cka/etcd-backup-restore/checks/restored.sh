#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

uid=$(kubectl -n vault get configmap golden -o 'jsonpath={.metadata.uid}' 2>/dev/null || true)
if ! ks_answer_check golden-uid "$uid"; then
  echo "configmap vault/golden is not the one from the snapshot" >&2
  exit 1
fi
encoded=$(kubectl -n vault get secret token -o 'jsonpath={.data.token}' 2>/dev/null || true)
token=$(printf '%s' "$encoded" | base64 --decode 2>/dev/null || true)
if ! ks_answer_check token "$token"; then
  echo "secret vault/token does not hold the value from the snapshot" >&2
  exit 1
fi
