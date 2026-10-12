#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Print a fingerprint of the base directory on controlplane: the names and the
# content of its files.
ks_node controlplane sh -c \
  'cd /opt/kustomize/metrics-server/base && find . -type f | sort | xargs sha256sum | sha256sum'
