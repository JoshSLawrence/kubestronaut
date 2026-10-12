#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_log "generating /root/jane.key on controlplane"
ks_node controlplane openssl genrsa -out /root/jane.key 2048
ks_node controlplane chmod 600 /root/jane.key
ks_node controlplane test -s /root/jane.key
