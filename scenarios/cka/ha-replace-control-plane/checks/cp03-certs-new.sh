#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

serial=$(ks_node controlplane03 openssl x509 -in /etc/kubernetes/pki/apiserver.crt -noout -serial 2>/dev/null) || {
  echo "controlplane03 has no API server certificate" >&2
  exit 1
}
if ks_answer_check cp03-cert "$serial"; then
  echo "controlplane03 still has the API server certificate from before it was wiped" >&2
  exit 1
fi
