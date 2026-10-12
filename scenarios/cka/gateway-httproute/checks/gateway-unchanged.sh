#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

uid=$(kubectl -n infra get gateway web-gw -o 'jsonpath={.metadata.uid}' 2>/dev/null || true)
if ! ks_answer_check gateway-uid "$uid"; then
  echo "Gateway infra/web-gw was deleted and recreated" >&2
  exit 1
fi
rule=$(kubectl -n infra get gateway web-gw -o \
  "jsonpath={.spec.listeners[*].port}/{.spec.listeners[*].allowedRoutes.namespaces.from}/{.spec.listeners[*].allowedRoutes.namespaces.selector.matchLabels['shared-gateway-access']}")
if [ "$rule" != "8000/Selector/true" ]; then
  echo "the listener of Gateway infra/web-gw was changed" >&2
  exit 1
fi
