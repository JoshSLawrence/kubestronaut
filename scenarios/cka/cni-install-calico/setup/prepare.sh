#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_log "placing the CNI manifests on controlplane"
ks_copy_to controlplane files /opt/cni
