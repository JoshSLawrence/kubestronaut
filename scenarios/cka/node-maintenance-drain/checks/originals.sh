#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

for obj in poddisruptionbudget/web-pdb daemonset/node-agent deployment/web deployment/cache; do
  uid=$(kubectl -n maint get "$obj" -o 'jsonpath={.metadata.uid}' 2>/dev/null || true)
  if ! ks_answer_check "uid-${obj//\//-}" "$uid"; then
    echo "$obj is not the original object" >&2
    exit 1
  fi
done
