#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

manifests=/etc/kubernetes/manifests
ks_node controlplane mv /etc/kubernetes/kube-scheduler.yaml.bak \
  "$manifests/kube-scheduler.yaml"
ks_node controlplane sed -i \
  's#--kubeconfig=/etc/kubernetes/controller-manager.config#--kubeconfig=/etc/kubernetes/controller-manager.conf#' \
  "$manifests/kube-controller-manager.yaml"
kubectl -n app rollout status deployment/web --timeout=240s
