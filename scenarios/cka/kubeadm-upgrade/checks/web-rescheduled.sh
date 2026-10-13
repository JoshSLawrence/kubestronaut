#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

uids=$(kubectl -n apps get pods -l app=web -o 'jsonpath={.items[*].metadata.uid}')
if [ -z "$uids" ]; then
  echo "no pod of apps/web exists" >&2
  exit 1
fi
for uid in $uids; do
  if ks_answer_check "web-pod-$uid" 1 2>/dev/null; then
    echo "a pod of apps/web from before the upgrade is still running" >&2
    exit 1
  fi
done

# Recreated pods alone prove nothing, since a rollout restart recreates them.
# Cordoning a node, which drain does first, makes its kubelet record this event.
for alias in controlplane node01; do
  name=$(ks_node_name "$alias")
  n=$(kubectl get events -A --field-selector "reason=NodeNotSchedulable,involvedObject.name=$name" -o name | grep -c . || true)
  if [ "$n" -lt 1 ]; then
    echo "$alias was never taken out of scheduling" >&2
    exit 1
  fi
done
