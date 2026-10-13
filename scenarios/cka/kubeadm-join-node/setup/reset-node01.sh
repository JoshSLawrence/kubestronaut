#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

w=$(ks_node_name node01)
# A node object that a later join replaces has a new uid; the check compares
# against this one, which nothing the learner does can bring back.
uid=$(kubectl get node "$w" -o 'jsonpath={.metadata.uid}')
ks_answer_put node01-uid "$uid"

kubectl drain "$w" --ignore-daemonsets --delete-emptydir-data --force
# Reset first: a running kubelet would register the node again as soon as the
# Node object is deleted.
ks_node node01 kubeadm reset -f
# kubeadm reset leaves the CNI configuration, and kind leaves the join
# configuration that holds its well-known token on every node.
ks_node node01 sh -c 'rm -rf /etc/cni/net.d/* /kind/kubeadm.conf'
kubectl delete node "$w"
# kind's join token never expires. Remove it so that only a token the
# learner creates can join the node.
ks_node controlplane kubeadm token delete abcdef.0123456789abcdef
if ks_node node01 systemctl is-active --quiet kubelet; then
  ks_log "kubelet on node01 is still running after the reset"
  exit 1
fi
