#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# The random marker means only the real volume can produce the expected value.
marker=$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')
ks_log "writing /srv/archive/marker.txt on controlplane"
ks_node controlplane mkdir -p /srv/archive
printf '%s' "$marker" |
  ks_node controlplane sh -c 'cat > /srv/archive/marker.txt'
ks_node controlplane test -s /srv/archive/marker.txt
ks_answer_put marker "$marker"
