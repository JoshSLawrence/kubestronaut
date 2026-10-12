#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

etcdctl_args=(
  --endpoints=https://127.0.0.1:2379
  --cacert=/etc/kubernetes/pki/etcd/ca.crt
  --cert=/etc/kubernetes/pki/etcd/server.crt
  --key=/etc/kubernetes/pki/etcd/server.key
)

ks_node controlplane etcdctl "${etcdctl_args[@]}" snapshot save /opt/backup/now.db
ks_node controlplane etcdutl snapshot restore /opt/backup/golden.db \
  --data-dir /var/lib/etcd-golden
ks_node controlplane sed -i \
  's#^\( *path: \)/var/lib/etcd$#\1/var/lib/etcd-golden#' \
  /etc/kubernetes/manifests/etcd.yaml

# The API server is down while etcd restarts with the restored data.
for _ in $(seq 90); do
  if kubectl get --raw /readyz >/dev/null 2>&1; then
    kubectl -n vault get configmap golden >/dev/null 2>&1 && exit 0
  fi
  sleep 3
done
echo "the API server did not come back within 270s" >&2
exit 1
