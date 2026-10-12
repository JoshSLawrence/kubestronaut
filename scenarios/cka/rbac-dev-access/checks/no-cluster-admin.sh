#!/usr/bin/env bash
set -euo pipefail

# A direct cluster-admin binding would also satisfy the other checks, so this
# one looks at the bindings rather than at behaviour.
sa="deployer"
bindings=$(kubectl get clusterrolebindings,rolebindings --all-namespaces \
  -o jsonpath='{range .items[?(@.roleRef.name=="cluster-admin")]}{range .subjects[*]}{.name}{"\n"}{end}{end}')
if grep -qx "$sa" <<<"$bindings"; then
  echo "the service account is bound to cluster-admin" >&2
  exit 1
fi
