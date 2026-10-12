#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

manifest=/etc/kubernetes/manifests/kube-apiserver.yaml
ks_node controlplane sed -i \
  -e 's#--etcd-servers=https://127.0.0.1:2380#--etcd-servers=https://127.0.0.1:2379#' \
  -e 's#--tls-private-key-file=/etc/kubernetes/pki/apiserver.pem#--tls-private-key-file=/etc/kubernetes/pki/apiserver.key#' \
  "$manifest"

for _ in $(seq 90); do
  if kubectl get --raw /readyz >/dev/null 2>&1; then exit 0; fi
  sleep 3
done
echo "the API server did not come back within 270s" >&2
exit 1
