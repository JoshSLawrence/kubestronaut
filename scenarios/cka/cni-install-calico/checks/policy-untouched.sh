#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

spec=$(kubectl -n secure get networkpolicy deny-from-net-test -o 'jsonpath={.spec}' 2>/dev/null || true)
if ! ks_answer_check policy-spec "$spec"; then
  echo "NetworkPolicy secure/deny-from-net-test is missing or was changed" >&2
  exit 1
fi
