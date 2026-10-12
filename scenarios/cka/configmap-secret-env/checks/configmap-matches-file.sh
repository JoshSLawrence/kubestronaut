#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The file on the node is the source of truth, so no expected value is stored.
want=$(ks_node controlplane cat /opt/config/app.properties)
got=$(kubectl -n config get configmap app-config \
  -o 'jsonpath={.data.app\.properties}')
if [ "$got" != "$want" ]; then
  echo "key app.properties does not match /opt/config/app.properties" >&2
  exit 1
fi
