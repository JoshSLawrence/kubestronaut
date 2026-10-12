#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

now=$(bash "$(dirname "$0")/base-fingerprint.sh" 2>/dev/null || true)
if ! ks_answer_check base "$now"; then
  echo "the base directory was changed" >&2
  exit 1
fi
