#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

want=$(ks_node controlplane cat /opt/config/app.properties)
got=$(kubectl -n config exec deploy/api -- cat /etc/app/app.properties)
if [ "$got" != "$want" ]; then
  echo "/etc/app/app.properties in a running pod does not match the file" >&2
  exit 1
fi
