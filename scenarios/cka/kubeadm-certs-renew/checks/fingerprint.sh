#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Print a fingerprint of the certificates named by $1, computed on the node:
#   apiserver        the API server's serving certificate
#   kubelet-client   the API server's client certificate for the kubelet
#   others           every other kubeadm certificate and kubeconfig, together
# Setup records these and the checks compare against them.
case "${1:?which certificate}" in
  apiserver)
    ks_node controlplane openssl x509 -in /etc/kubernetes/pki/apiserver.crt -noout -fingerprint -sha256
    ;;
  kubelet-client)
    ks_node controlplane openssl x509 -in /etc/kubernetes/pki/apiserver-kubelet-client.crt -noout -fingerprint -sha256
    ;;
  others)
    # kubelet.conf is left out: the kubelet rotates its own certificate.
    ks_node controlplane sh <<'SH'
cd /etc/kubernetes
for f in $(find pki -name "*.crt" | sort); do
  case "$f" in
    pki/apiserver.crt | pki/apiserver-kubelet-client.crt) continue ;;
  esac
  cat "$f"
done | sha256sum
cat admin.conf controller-manager.conf scheduler.conf super-admin.conf | sha256sum
SH
    ;;
  *)
    echo "unknown certificate $1" >&2
    exit 2
    ;;
esac
