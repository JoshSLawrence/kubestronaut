#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Two components, one fault each: the scheduler's static pod manifest is moved
# out of the manifests directory, and the controller manager is pointed at a
# kubeconfig file that does not exist. Nothing here touches the API server, so
# kubectl keeps working.
manifests=/etc/kubernetes/manifests
ks_log "removing the scheduler manifest and breaking the controller manager"
ks_node controlplane mv "$manifests/kube-scheduler.yaml" \
  /etc/kubernetes/kube-scheduler.yaml.bak
ks_node controlplane sed -i \
  's#--kubeconfig=/etc/kubernetes/controller-manager.conf#--kubeconfig=/etc/kubernetes/controller-manager.config#' \
  "$manifests/kube-controller-manager.yaml"
ks_node controlplane grep -q 'controller-manager.config' \
  "$manifests/kube-controller-manager.yaml"

# Wait until both faults show: no scheduler pod, and a controller manager that
# has crashed and been restarted at least once, so the first grade is stable.
# A slow machine gets plenty of time: the faults stay in place until the
# learner fixes them.
for _ in $(seq 90); do
  sched=$(kubectl -n kube-system get pods -l component=kube-scheduler \
    -o 'jsonpath={.items[*].metadata.name}')
  restarts=$(kubectl -n kube-system get pods -l component=kube-controller-manager \
    -o 'jsonpath={.items[*].status.containerStatuses[0].restartCount}')
  ready=$(kubectl -n kube-system get pods -l component=kube-controller-manager \
    -o 'jsonpath={.items[*].status.conditions[?(@.type=="Ready")].status}')
  # No controller manager pod for a moment (the kubelet is recreating it)
  # leaves both values empty: keep waiting.
  if [ -n "$restarts" ] && [ -z "$sched" ] && [ "$restarts" -ge 1 ] && [ "$ready" != "True" ]; then
    exit 0
  fi
  sleep 2
done
echo "the control plane pods did not change state within 180s" >&2
exit 1
