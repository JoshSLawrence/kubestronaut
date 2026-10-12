#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

cp=$(ks_node_name controlplane)
w=$(ks_node_name node01)
version=1.37.1-1.1
apt='DEBIAN_FRONTEND=noninteractive apt-get'

# Upgrading the kubelet makes it recreate the control-plane static pods one
# after another, so the API server answers for a while and then restarts. Until
# it has synced RBAC again, requests are rejected as forbidden, which makes a
# drain fail halfway. Wait until it has answered several times in a row.
wait_api() {
  local _ ok=0
  for _ in $(seq 100); do
    if [ "$(kubectl auth can-i list pods -A 2>/dev/null)" = yes ]; then
      ok=$((ok + 1))
      if [ "$ok" -ge 8 ]; then return 0; fi
    else
      ok=0
    fi
    sleep 3
  done
  echo "the API server did not stay up" >&2
  return 1
}

# Control plane: kubeadm, plan, apply.
ks_node controlplane sh -c "$apt update && apt-mark unhold kubeadm && $apt install -y kubeadm=$version && apt-mark hold kubeadm"
ks_node controlplane kubeadm version -o short
ks_node controlplane kubeadm upgrade plan
ks_node controlplane kubeadm upgrade apply v1.37.1 -y
wait_api

# Control plane: drain, kubelet and kubectl, restart, uncordon.
kubectl drain "$cp" --ignore-daemonsets --delete-emptydir-data
ks_node controlplane sh -c "apt-mark unhold kubelet kubectl; $apt install -y kubelet=$version kubectl=$version && apt-mark hold kubelet kubectl"
ks_node controlplane sh -c 'systemctl daemon-reload && systemctl restart kubelet'
ks_wait_node controlplane Ready 180
wait_api
kubectl uncordon "$cp"

# Worker: kubeadm, upgrade node, drain, kubelet and kubectl, restart, uncordon.
ks_node node01 sh -c "$apt update && apt-mark unhold kubeadm && $apt install -y kubeadm=$version && apt-mark hold kubeadm"
ks_node node01 kubeadm upgrade node
kubectl drain "$w" --ignore-daemonsets --delete-emptydir-data
ks_node node01 sh -c "apt-mark unhold kubelet kubectl; $apt install -y kubelet=$version kubectl=$version && apt-mark hold kubelet kubectl"
ks_node node01 sh -c 'systemctl daemon-reload && systemctl restart kubelet'
ks_wait_node node01 Ready 180
wait_api
kubectl uncordon "$w"
kubectl -n apps rollout status deployment/web --timeout=180s
