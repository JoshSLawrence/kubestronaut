#!/usr/bin/env bash
set -euo pipefail

# A literal IP in a hosts block would resolve today but not after the Service
# is recreated, so the Corefile must map the name to the Service's DNS name.
corefile=$(kubectl -n kube-system get configmap coredns -o 'jsonpath={.data.Corefile}')
if ! grep -Eq '^[[:space:]]*rewrite[[:space:]]+(stop[[:space:]]+|continue[[:space:]]+)?name[[:space:]]+(exact[[:space:]]+)?legacy\.example\.internal[[:space:]]+legacy\.apps\.svc\.cluster\.local\.?[[:space:]]*$' <<<"$corefile"; then
  echo "the Corefile does not map the name to the Service's DNS name" >&2
  exit 1
fi
