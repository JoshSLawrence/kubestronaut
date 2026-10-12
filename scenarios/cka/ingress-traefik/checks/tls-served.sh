#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ips=$(kubectl get node "$KS_NODE_CONTROLPLANE" \
  -o 'jsonpath={.status.addresses[?(@.type=="InternalIP")].address}')
ip=${ips%% *}

# Ask for the certificate the way a browser does, with the host name as SNI.
chain=$(ks_node controlplane openssl s_client -connect "$ip:30443" \
  -servername shop.example.local </dev/null 2>/dev/null) || chain=
served=$(printf '%s' "$chain" | ks_node controlplane openssl x509 -noout -fingerprint -sha256 2>/dev/null) || served=
if ! ks_answer_check shop-cert "$served"; then
  echo "port 30443 does not serve the certificate from /opt/certs for shop.example.local" >&2
  exit 1
fi
