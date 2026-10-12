#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

kubectl -n maint rollout status daemonset/node-agent --timeout=120s
node=$(ks_node_name node01)
# The scenario is only a task if node01 really holds pods to move.
for app in web cache; do
  n=$(kubectl -n maint get pods -l "app=$app" \
    --field-selector "spec.nodeName=$node" -o name | grep -c . || true)
  if [ "$n" -lt 1 ]; then
    echo "no $app pod landed on node01: stop and start again" >&2
    exit 1
  fi
done

# Deleting and recreating these gives them a new uid, which is how a check
# notices that a protected object was removed to make the drain easier.
for obj in poddisruptionbudget/web-pdb daemonset/node-agent deployment/web deployment/cache; do
  uid=$(kubectl -n maint get "$obj" -o 'jsonpath={.metadata.uid}')
  ks_answer_put "uid-${obj//\//-}" "$uid"
done
spec=$(kubectl -n maint get pod scratch -o \
  'jsonpath={.spec.containers[0].image}{"|"}{.spec.containers[0].command}')
ks_answer_put scratch-spec "$spec"
