#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

cp3=$(ks_node_name controlplane03)
etcdctl_env='ETCDCTL_ENDPOINTS=https://127.0.0.1:2379 ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt ETCDCTL_CERT=/etc/kubernetes/pki/etcd/server.crt ETCDCTL_KEY=/etc/kubernetes/pki/etcd/server.key'

id=$(ks_node controlplane sh -c "$etcdctl_env etcdctl member list" | awk -F', ' -v n="$cp3" '$3 == n {print $1}')
if [ -z "$id" ]; then
  ks_log "etcd has no member named $cp3"
  exit 1
fi
# What a rebuilt node must not still have: the old etcd member, the old Node
# object and the old serving certificate. Only hashes are kept.
ks_answer_put stale-member "$id"
ks_answer_put cp03-uid "$(kubectl get node "$cp3" -o 'jsonpath={.metadata.uid}')"
ks_answer_put cp03-cert "$(ks_node controlplane03 openssl x509 -in /etc/kubernetes/pki/apiserver.crt -noout -serial)"

# Wipe the node but leave its etcd member behind, as a dead disk would.
ks_node controlplane03 kubeadm reset -f --skip-phases=remove-etcd-member
ks_node controlplane03 sh -c 'rm -rf /var/lib/etcd /etc/cni/net.d/* /kind/kubeadm.conf && mkdir -p /var/lib/etcd'
ks_wait_node controlplane03 NotReady 180
