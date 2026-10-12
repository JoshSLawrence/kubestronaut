#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The pod can run on any schedulable node, so look where it actually is.
node=$(kubectl -n app get pod logger -o 'jsonpath={.spec.nodeName}')
for var in $(compgen -A variable KS_NODE_); do
  if [ "${!var}" = "$node" ]; then
    alias=$(tr '[:upper:]' '[:lower:]' <<<"${var#KS_NODE_}")
    ks_node "$alias" test -s /mnt/logs/out.log ||
      {
        echo "no log file under /mnt/logs on the pod's node" >&2
        exit 1
      }
    exit 0
  fi
done
echo "the logger pod is not running on any node" >&2
exit 1
