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

# What golden.db will hold: a configmap and a secret with values only this
# session knows, so recreating them by hand cannot pass.
token=$(openssl rand -hex 16)
kubectl create namespace vault
kubectl -n vault create configmap golden --from-literal=purpose=restore-me
kubectl -n vault create secret generic token --from-literal=token="$token"
ks_answer_put golden-uid "$(kubectl -n vault get configmap golden -o 'jsonpath={.metadata.uid}')"
ks_answer_put token "$token"

ks_log "taking the earlier snapshot"
ks_node controlplane mkdir -p /opt/backup
ks_node controlplane etcdctl "${etcdctl_args[@]}" snapshot save /opt/backup/golden.db

# Changes after the snapshot: restoring golden.db must undo them.
kubectl -n vault delete configmap golden
kubectl -n vault delete secret token
kubectl -n vault create configmap post-snapshot --from-literal=purpose=undo-me
