#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_log "placing the operator manifest on controlplane"
ks_copy_to controlplane files /opt/operators
