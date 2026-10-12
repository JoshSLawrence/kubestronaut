#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

got=$(kubectl -n reports exec archiver -- cat /archive/marker.txt 2>/dev/null) ||
  {
    echo "cannot read /archive/marker.txt in the pod" >&2
    exit 1
  }
if ! ks_answer_check marker "$got"; then
  echo "/archive does not hold the original data" >&2
  exit 1
fi
