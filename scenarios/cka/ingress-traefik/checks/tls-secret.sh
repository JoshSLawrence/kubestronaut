#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

type=$(kubectl -n shop get secret shop-tls -o 'jsonpath={.type}' 2>/dev/null || true)
if [ "$type" != "kubernetes.io/tls" ]; then
  echo "Secret shop/shop-tls is missing or is not of type kubernetes.io/tls" >&2
  exit 1
fi
crt=$(kubectl -n shop get secret shop-tls -o 'jsonpath={.data.tls\.crt}')
fp=$(printf '%s' "$crt" | ks_node controlplane bash -c 'base64 -d | openssl x509 -noout -fingerprint -sha256')
if ! ks_answer_check shop-cert "$fp"; then
  echo "Secret shop/shop-tls does not hold the certificate from /opt/certs/shop.crt" >&2
  exit 1
fi
