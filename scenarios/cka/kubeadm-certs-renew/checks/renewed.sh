#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

now=$(bash "$(dirname "$0")/fingerprint.sh" "$1")
if ks_answer_check "cert-$1" "$now"; then
  echo "the certificate has not been renewed" >&2
  exit 1
fi
