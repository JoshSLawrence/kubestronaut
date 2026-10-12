#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_log "placing the upstream manifests on controlplane"
ks_copy_to controlplane files/base /opt/kustomize/metrics-server/base
# Any change to the directory, including a new file, changes this fingerprint.
ks_answer_put base "$(bash checks/base-fingerprint.sh)"
