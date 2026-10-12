#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

spec=$(kubectl -n secure get networkpolicy deny-from-net-test -o 'jsonpath={.spec}')
ks_answer_put policy-spec "$spec"
