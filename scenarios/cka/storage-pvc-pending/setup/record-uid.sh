#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Recreating the volume gives it a new uid, which is how a check notices.
uid=$(kubectl get pv archive-pv -o 'jsonpath={.metadata.uid}')
ks_answer_put pv-uid "$uid"
ks_log "volume archive-pv has uid $uid"
