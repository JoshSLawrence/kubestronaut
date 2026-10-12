#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Wait for the first restart, so that the pod is crash looping when the learner
# starts, and record the last line it logs. The line is printed as soon as an
# instance starts, so plain logs hold it too when --previous has nothing.
want='^FATAL [0-9a-f]\{8\}: DB_HOST not set$'
line=""
for _ in $(seq 90); do
  restarts=$(kubectl -n orders get pods -l app=orders \
    -o 'jsonpath={.items[0].status.containerStatuses[0].restartCount}')
  if [ "${restarts:-0}" -ge 1 ]; then
    line=$(kubectl -n orders logs deployment/orders --previous 2>/dev/null | tail -n1 || true)
    if ! grep -q "$want" <<<"$line"; then
      line=$(kubectl -n orders logs deployment/orders 2>/dev/null | tail -n1 || true)
    fi
    grep -q "$want" <<<"$line" && break
  fi
  sleep 2
done
if ! grep -q "$want" <<<"$line"; then
  echo "the app did not crash with the expected message" >&2
  exit 1
fi
ks_answer_put error-line "$line"

spec=$(kubectl -n orders get deployment orders -o \
  'jsonpath={.spec.template.spec.containers[?(@.name=="orders")].image}{"|"}{.spec.template.spec.containers[?(@.name=="orders")].command}{"|"}{.spec.template.spec.containers[?(@.name=="orders")].args}')
ks_answer_put container-spec "$spec"
