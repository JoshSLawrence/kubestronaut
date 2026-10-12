#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

ks_node controlplane sh -c 'kubeadm certs check-expiration > /root/cert-expiry.txt 2>/dev/null'
ks_node controlplane kubeadm certs renew apiserver
ks_node controlplane kubeadm certs renew apiserver-kubelet-client

# Moving the manifest away stops the static pod; moving it back starts it with
# the new certificates.
manifest=/etc/kubernetes/manifests/kube-apiserver.yaml
ks_node controlplane mv "$manifest" /etc/kubernetes/kube-apiserver.yaml.bak
for _ in $(seq 30); do
  if ! kubectl get --raw /readyz >/dev/null 2>&1; then break; fi
  sleep 2
done
ks_node controlplane mv /etc/kubernetes/kube-apiserver.yaml.bak "$manifest"
for _ in $(seq 60); do
  if kubectl get --raw /readyz >/dev/null 2>&1; then exit 0; fi
  sleep 3
done
echo "the API server did not come back" >&2
exit 1
