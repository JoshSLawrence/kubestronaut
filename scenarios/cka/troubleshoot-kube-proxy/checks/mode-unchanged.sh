#!/usr/bin/env bash
set -euo pipefail

# Read the whole ConfigMap first: "grep -q" exits early and would make kubectl
# fail with a broken pipe under pipefail.
conf=$(kubectl -n kube-system get configmap kube-proxy -o 'jsonpath={.data.config\.conf}')
if ! grep -Eq '^mode: iptables[[:space:]]*$' <<<"$conf"; then
  echo "the proxy mode in the kube-proxy ConfigMap was changed" >&2
  exit 1
fi
