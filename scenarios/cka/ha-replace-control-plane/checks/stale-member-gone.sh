#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

etcdctl_env='ETCDCTL_ENDPOINTS=https://127.0.0.1:2379 ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt ETCDCTL_CERT=/etc/kubernetes/pki/etcd/server.crt ETCDCTL_KEY=/etc/kubernetes/pki/etcd/server.key'
members=$(ks_node controlplane sh -c "$etcdctl_env etcdctl member list") || {
  echo "etcd did not answer on controlplane" >&2
  exit 1
}
for id in $(printf '%s\n' "$members" | awk -F', ' '{print $1}'); do
  if ks_answer_check stale-member "$id"; then
    echo "the etcd member of the lost node is still registered" >&2
    exit 1
  fi
done
