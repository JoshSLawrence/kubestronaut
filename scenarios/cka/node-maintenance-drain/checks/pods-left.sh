#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=/dev/null
source "$KS_LIB"

# Go by the owner, not by a label: a pod can carry any label, but only the
# DaemonSet's own pods are allowed to stay.
node=$(ks_node_name node01)
pods=$(kubectl -n maint get pods --field-selector "spec.nodeName=$node" -o \
  'jsonpath={range .items[*]}{.metadata.name}{" "}{.metadata.ownerReferences[0].kind}/{.metadata.ownerReferences[0].name}{"\n"}{end}')
stay=$(grep -c . <<<"$(grep -v ' DaemonSet/node-agent$' <<<"$pods" || true)" || true)
if [ "$stay" -ne 0 ]; then
  echo "pods other than the DaemonSet's still run on node01" >&2
  exit 1
fi
