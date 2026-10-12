#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The check compares image and command with what setup created.
spec=$(kubectl -n logging get deployment legacy-app -o \
  'jsonpath={.spec.template.spec.containers[?(@.name=="app")].image}{"|"}{.spec.template.spec.containers[?(@.name=="app")].command}')
ks_answer_put app-spec "$spec"
