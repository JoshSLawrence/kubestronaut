#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# One fault: kube-proxy is told to read a config file that does not exist.
# The pods are deleted so that every node runs the broken version; a rolling
# update would stop at the first node and leave the other one working.
kubectl -n kube-system patch daemonset kube-proxy --type=json -p \
  '[{"op":"replace","path":"/spec/template/spec/containers/0/command/1","value":"--config=/var/lib/kube-proxy/kubeproxy.conf"}]'
kubectl -n kube-system delete pods -l k8s-app=kube-proxy --wait=true

# Wait for the new pods to exist and to be crashing, on both nodes.
for _ in $(seq 90); do
  state=$(kubectl -n kube-system get pods -l k8s-app=kube-proxy -o \
    'jsonpath={range .items[*]}{.spec.containers[0].command[1]}{" "}{.status.containerStatuses[0].ready}{"\n"}{end}')
  total=$(grep -c . <<<"$state" || true)
  bad=$(grep -c 'kubeproxy\.conf false' <<<"$state" || true)
  if [ "$total" -eq 2 ] && [ "$bad" -eq 2 ]; then
    break
  fi
  sleep 2
done
if [ "${bad:-0}" -ne 2 ]; then
  echo "kube-proxy did not fail on both nodes within 180s" >&2
  exit 1
fi

# Remove every kube-proxy NAT rule so that the Services stop working at once,
# and leave the other NAT rules (Docker's DNS, kind's masquerading) alone.
for node in controlplane node01; do
  ks_log "removing the kube-proxy NAT rules on $node"
  ks_node "$node" sh -c 'iptables-save -t nat | grep -v KUBE- | iptables-restore'
  left=$(ks_node "$node" sh -c 'iptables-save -t nat | grep -c KUBE-SVC || true')
  if [ "${left//[[:space:]]/}" != "0" ]; then
    echo "kube-proxy NAT rules are still present on $node" >&2
    exit 1
  fi
done
