#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

cp3=$(ks_node_name controlplane03)
etcdctl_env='ETCDCTL_ENDPOINTS=https://127.0.0.1:2379 ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt ETCDCTL_CERT=/etc/kubernetes/pki/etcd/server.crt ETCDCTL_KEY=/etc/kubernetes/pki/etcd/server.key'

# 1. Remove the dead member from etcd, then the Node object.
id=$(ks_node controlplane sh -c "$etcdctl_env etcdctl member list" | awk -F', ' -v n="$cp3" '$3 == n {print $1}')
ks_node controlplane sh -c "$etcdctl_env etcdctl member remove $id"
kubectl delete node "$cp3"

# 2. Upload the control-plane certificates, create a join command that carries
# the certificate key, and run it on the wiped node.
key=$(ks_node controlplane kubeadm init phase upload-certs --upload-certs | tail -n 1)
join=$(ks_node controlplane kubeadm token create --print-join-command --certificate-key "$key")
ks_node controlplane03 sh -c "$join"
ks_wait_node controlplane03 Ready 240
