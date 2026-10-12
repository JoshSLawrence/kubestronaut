#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

now=$(bash "$(dirname "$0")/fingerprint.sh" others)
if ! ks_answer_check cert-others "$now"; then
  echo "a certificate or kubeconfig other than the two asked for changed" >&2
  exit 1
fi
