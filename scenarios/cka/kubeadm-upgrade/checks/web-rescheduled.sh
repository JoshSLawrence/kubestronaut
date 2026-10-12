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
  if [ -f "$KS_SCENARIO_DIR/.answers/web-pod-$uid" ]; then
    echo "a pod of apps/web from before the upgrade is still running" >&2
    exit 1
  fi
done
