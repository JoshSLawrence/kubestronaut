#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Usage: resolves.sh <name> <namespace> <service>
# Ask the cluster DNS Service directly from the node, so a broken DNS fails
# this check cleanly instead of failing the way a probe from a pod would.
name=$1 ns=$2 svc=$3
want=$(kubectl -n "$ns" get service "$svc" -o 'jsonpath={.spec.clusterIP}')
got=$(ks_node controlplane dig +short +time=2 +tries=1 @10.96.0.10 "$name" 2>/dev/null || true)
got=${got%%$'\n'*}
if [ -z "$want" ] || [ "$got" != "$want" ]; then
  echo "$name does not resolve to the Service address" >&2
  exit 1
fi
