#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

spec=$(kubectl -n orders get deployment orders -o \
  'jsonpath={.spec.template.spec.containers[?(@.name=="orders")].image}{"|"}{.spec.template.spec.containers[?(@.name=="orders")].command}{"|"}{.spec.template.spec.containers[?(@.name=="orders")].args}')
if ! ks_answer_check container-spec "$spec"; then
  echo "the image or command of the orders container changed" >&2
  exit 1
fi
