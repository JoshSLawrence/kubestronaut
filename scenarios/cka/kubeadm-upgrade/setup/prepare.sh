#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

for n in controlplane node01; do
  v=$(ks_node "$n" kubeadm version -o short)
  if [ "$v" != v1.36.4 ]; then
    ks_log "$n runs kubeadm $v, but this lab upgrades from v1.36.4: start it without --k8s-version"
    exit 1
  fi
  if ! ks_node "$n" test -s /opt/kubernetes-apt/v1.37/Packages; then
    ks_log "$n has no offline v1.37 apt repository: start without --node-image so kubestronaut builds its own node image"
    exit 1
  fi
  # Offline-safe apt: the local mirror is the only package source, so an
  # update does not wait for a network that may not be there.
  ks_node "$n" sh -c 'cp /opt/kubernetes-apt/kubernetes.list /etc/apt/sources.list.d/kubernetes.list &&
    mv /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list.d/debian.sources.disabled'
  # kind keeps its own /etc/default/kubelet (--runtime-cgroups), which the
  # kubelet package would ask about. Keep the existing file without a prompt;
  # only this lab changes dpkg's behaviour.
  ks_node "$n" sh -c 'echo force-confold > /etc/dpkg/dpkg.cfg.d/kubestronaut-confold'
done

# kubeadm upgrade apply wants these images. They are preloaded through
# "images:", but Docker cannot always export them; pulling in the node is the
# fallback, and kubeadm itself would pull as a last resort.
for img in registry.k8s.io/kube-apiserver:v1.37.1 registry.k8s.io/kube-controller-manager:v1.37.1 \
  registry.k8s.io/kube-scheduler:v1.37.1 registry.k8s.io/kube-proxy:v1.37.1 \
  registry.k8s.io/coredns/coredns:v1.14.6 registry.k8s.io/etcd:3.7.0-0; do
  if ! ks_node controlplane crictl inspecti "$img" >/dev/null 2>&1; then
    ks_log "$img was not preloaded: pulling it on controlplane"
    ks_node controlplane crictl pull "$img" >/dev/null ||
      ks_log "warning: $img is not on controlplane and could not be pulled. Reconnect to the internet and run: kubestronaut ssh controlplane -- crictl pull $img (or run kubestronaut reset while online); otherwise kubeadm upgrade apply cannot fetch it"
  fi
done
if ! ks_node node01 crictl inspecti registry.k8s.io/kube-proxy:v1.37.1 >/dev/null 2>&1; then
  ks_node node01 crictl pull registry.k8s.io/kube-proxy:v1.37.1 >/dev/null ||
    ks_log "warning: kube-proxy:v1.37.1 is not on node01 and could not be pulled. Reconnect to the internet and run: kubestronaut ssh node01 -- crictl pull registry.k8s.io/kube-proxy:v1.37.1 (or run kubestronaut reset while online)"
fi

# The upgrade must evict these pods from node01 at some point. Pods that a
# drain did not recreate keep their uid, so this is how a check can tell that
# node01 was drained.
for uid in $(kubectl -n apps get pods -l app=web -o 'jsonpath={.items[*].metadata.uid}'); do
  ks_answer_put "web-pod-$uid" 1
done
