#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Two faults that show one after the other: the key file named by the manifest
# does not exist, so the server exits at once; once that is fixed it starts but
# cannot reach etcd, because 2380 is etcd's peer port, not its client port.
manifest=/etc/kubernetes/manifests/kube-apiserver.yaml
ks_log "breaking the API server"
ks_node controlplane sed -i \
  -e 's#--etcd-servers=https://127.0.0.1:2379#--etcd-servers=https://127.0.0.1:2380#' \
  -e 's#--tls-private-key-file=/etc/kubernetes/pki/apiserver.key#--tls-private-key-file=/etc/kubernetes/pki/apiserver.pem#' \
  "$manifest"
ks_node controlplane grep -q 'apiserver.pem' "$manifest"
ks_node controlplane grep -q 'etcd-servers=https://127.0.0.1:2380' "$manifest"

# The faults stay until the learner fixes them, so a slow machine can take its
# time. Wait for the API to go away so the first grade is stable.
for _ in $(seq 90); do
  if ! kubectl get --raw /readyz >/dev/null 2>&1; then
    exit 0
  fi
  sleep 2
done
echo "the API server did not stop within 180s" >&2
exit 1
