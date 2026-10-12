#!/usr/bin/env bash
set -euo pipefail

# A crashing kube-proxy is "Ready" for a moment after each start, so one look
# is not enough. The pods must be ready, and be the same containers with the
# same restart counts, a few seconds apart.
snapshot() {
  kubectl -n kube-system get daemonset kube-proxy -o \
    'jsonpath={.status.desiredNumberScheduled}/{.status.updatedNumberScheduled}/{.status.numberReady}{"\n"}'
  kubectl -n kube-system get pods -l k8s-app=kube-proxy -o \
    'jsonpath={range .items[*]}{.metadata.name}:{.status.containerStatuses[0].containerID}:{.status.containerStatuses[0].restartCount}:{.status.containerStatuses[0].ready}{"\n"}{end}'
}

first=$(snapshot)
sleep 6
second=$(snapshot)

counts=${first%%$'\n'*}
IFS=/ read -r desired updated ready <<<"$counts"
if [ -z "$desired" ] || [ "$desired" -lt 1 ] || [ "$desired" != "$updated" ] || [ "$desired" != "$ready" ]; then
  echo "kube-proxy is not running on every node" >&2
  exit 1
fi
if grep -q ':false$' <<<"$second" || [ "$first" != "$second" ]; then
  echo "kube-proxy is restarting" >&2
  exit 1
fi
