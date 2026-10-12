#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

spec=$(kubectl -n logging get deployment legacy-app -o \
  'jsonpath={.spec.template.spec.containers[?(@.name=="app")].image}{"|"}{.spec.template.spec.containers[?(@.name=="app")].command}')
if ! ks_answer_check app-spec "$spec"; then
  echo "the image or command of the app container changed" >&2
  exit 1
fi
