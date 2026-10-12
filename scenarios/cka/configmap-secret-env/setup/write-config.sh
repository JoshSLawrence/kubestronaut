#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The random build id means a hand-typed ConfigMap cannot match the file.
build_id=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
ks_log "writing /opt/config/app.properties on controlplane"
ks_node controlplane mkdir -p /opt/config
printf 'log.level=info\nfeature.cache=true\nbuild.id=%s\nregion=eu-west\n' \
  "$build_id" |
  ks_node controlplane sh -c 'cat > /opt/config/app.properties'
ks_node controlplane test -s /opt/config/app.properties
